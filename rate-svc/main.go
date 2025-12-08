package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"github.com/segmentio/kafka-go"
)

type Review struct {
	ID           int       `json:"id"`
	DishID       int       `json:"dish_id"`
	OrderID      int       `json:"order_id"`
	RestaurantID int       `json:"restaurant_id"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type KafkaMessage struct {
	Type         string    `json:"type"`
	DishID       int       `json:"dish_id"`
	RestaurantID int       `json:"restaurant_id"`
	OrderID      int       `json:"order_id"`
	Rating       int       `json:"rating"`
	Timestamp    time.Time `json:"timestamp"`
}

var (
	db          *sql.DB
	rdb         *redis.Client
	kafkaWriter *kafka.Writer
	ctx         = context.Background()

	errDishNotInOrder  = errors.New("dish was not ordered for this check")
	errDuplicateReview = errors.New("review already exists for this dish and check")
)

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}
}

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
}

func initKafka() {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "reviews",
		Balancer: &kafka.LeastBytes{},
	}
}

// Check if user already reviewed this dish in this order
func hasUserReviewedDish(dishID, orderID int) (bool, error) {
	cacheKey := fmt.Sprintf("review:%d:%d", dishID, orderID)
	exists, err := rdb.Exists(ctx, cacheKey).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// Validate that dish was actually ordered
func validateDishInOrder(dishID, orderID, restaurantID int) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM order_items oi
			JOIN orders o ON oi.order_id = o.id
			WHERE oi.dish_id = $1 AND oi.order_id = $2 AND o.restaurant_id = $3
		)
	`, dishID, orderID, restaurantID).Scan(&exists)

	return exists, err
}

func createReview(w http.ResponseWriter, r *http.Request) {
	var review Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := persistReview(&review); err != nil {
		switch {
		case errors.Is(err, errDishNotInOrder):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, errDuplicateReview):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

func persistReview(review *Review) error {
	// Validate that the dish was actually ordered in this check/restaurant
	valid, err := validateDishInOrder(review.DishID, review.OrderID, review.RestaurantID)
	if err != nil {
		return fmt.Errorf("failed to validate order: %w", err)
	}
	if !valid {
		return errDishNotInOrder
	}

	// Check if review exists in database (reliable source of truth)
	var existingReviewID int
	err = db.QueryRow(`
		SELECT id FROM reviews
		WHERE dish_id = $1 AND order_id = $2 AND restaurant_id = $3
	`, review.DishID, review.OrderID, review.RestaurantID).Scan(&existingReviewID)

	var isUpdate bool
	if err == nil {
		// Review exists — perform UPDATE
		isUpdate = true
		_, err = db.Exec(`
			UPDATE reviews
			SET rating = $1, comment = $2, created_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`, review.Rating, review.Comment, existingReviewID)
		if err != nil {
			return fmt.Errorf("failed to update review: %w", err)
		}
		review.ID = existingReviewID
	} else if err == sql.ErrNoRows {
		// No review exists — perform INSERT
		isUpdate = false
		err = db.QueryRow(`
			INSERT INTO reviews (dish_id, order_id, restaurant_id, rating, comment)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at
		`, review.DishID, review.OrderID, review.RestaurantID, review.Rating, review.Comment).Scan(&review.ID, &review.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert review: %w", err)
		}
	} else {
		return fmt.Errorf("failed to check existing review: %w", err)
	}

	// Update Redis cache in both cases (INSERT or UPDATE)
	cacheKey := fmt.Sprintf("review:%d:%d", review.DishID, review.OrderID)
	if err := rdb.Set(ctx, cacheKey, "1", 24*7*time.Hour).Err(); err != nil {
		log.Printf("Warning: failed to cache review marker: %v", err)
	}

	// Emit Kafka message for analytics recalculation (both cases)
	if kafkaWriter == nil {
		log.Printf("Warning: kafkaWriter is nil, skipping Kafka publish")
	} else {
		message := KafkaMessage{
			Type:         "new_review",
			DishID:       review.DishID,
			RestaurantID: review.RestaurantID,
			OrderID:      review.OrderID,
			Rating:       review.Rating,
			Timestamp:    time.Now(),
		}

		messageJSON, _ := json.Marshal(message)
		if err := kafkaWriter.WriteMessages(ctx, kafka.Message{
			Key:   []byte(strconv.Itoa(review.DishID)),
			Value: messageJSON,
		}); err != nil {
			return fmt.Errorf("failed to emit kafka message: %w", err)
		}
	}

	log.Printf("Successfully %s review for dish %d in order %d",
		map[bool]string{true: "updated", false: "created"}[isUpdate],
		review.DishID, review.OrderID)
	return nil
}

func getDishReviews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dishID, _ := strconv.Atoi(vars["dishId"])
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	rows, err := db.Query(`
		SELECT id, dish_id, order_id, restaurant_id, rating, comment, created_at
		FROM reviews
		WHERE dish_id = $1 AND restaurant_id = $2
		ORDER BY created_at DESC
	`, dishID, restaurantID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var reviews []Review
	for rows.Next() {
		var review Review
		rows.Scan(&review.ID, &review.DishID, &review.OrderID, &review.RestaurantID, &review.Rating, &review.Comment, &review.CreatedAt)
		reviews = append(reviews, review)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}

func createBulkReviews(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CheckID      int `json:"check_id"`
		RestaurantID int `json:"restaurant_id"`
		Reviews      []struct {
			DishID  int    `json:"dish_id"`
			Rating  int    `json:"rating"`
			Comment string `json:"comment"`
		} `json:"reviews"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if payload.CheckID == 0 || payload.RestaurantID == 0 || len(payload.Reviews) == 0 {
		http.Error(w, "Missing check_id, restaurant_id or reviews", http.StatusBadRequest)
		return
	}

	type reviewResult struct {
		DishID  int    `json:"dish_id"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}

	results := make([]reviewResult, 0, len(payload.Reviews))
	successCount := 0

	for _, incoming := range payload.Reviews {
		review := Review{
			DishID:       incoming.DishID,
			OrderID:      payload.CheckID,
			RestaurantID: payload.RestaurantID,
			Rating:       incoming.Rating,
			Comment:      incoming.Comment,
		}

		err := persistReview(&review)
		if err != nil {
			results = append(results, reviewResult{
				DishID:  incoming.DishID,
				Status:  "error",
				Message: err.Error(),
			})
			// Continue processing other reviews even if one fails
			continue
		}

		successCount++
		results = append(results, reviewResult{
			DishID: incoming.DishID,
			Status: "ok",
		})
	}

	if successCount == 0 {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed": results,
		"created":   successCount,
		"failed":    len(results) - successCount,
	})
}

func main() {
	initDB()
	defer db.Close()

	initRedis()
	defer rdb.Close()

	initKafka()
	defer kafkaWriter.Close()

	r := mux.NewRouter()

	// Rate service routes
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/reviews", createReview).Methods("POST")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/reviews", getDishReviews).Methods("GET")
	r.HandleFunc("/api/reviews", createBulkReviews).Methods("POST")

	handler := cors.Default().Handler(r)

	log.Println("Rate Service starting on port 8082")
	log.Fatal(http.ListenAndServe(":8082", handler))
}
