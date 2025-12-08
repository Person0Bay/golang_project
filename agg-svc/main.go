package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	_ "github.com/lib/pq"
)

type KafkaMessage struct {
	Type         string    `json:"type"`
	DishID       int       `json:"dish_id"`
	RestaurantID int       `json:"restaurant_id"`
	OrderID      int       `json:"order_id"`
	Rating       int       `json:"rating"`
	Timestamp    time.Time `json:"timestamp"`
}

var (
	db   *sql.DB
	rdb  *redis.Client
	ctx  = context.Background()
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

func updateDishRating(dishID, restaurantID, rating int) error {
	// Update aggregates in PostgreSQL.
	_, err := db.Exec(`
		UPDATE dishes
		SET avg_rating = (
			SELECT ROUND(AVG(rating::numeric), 2)
			FROM reviews
			WHERE dish_id = $1
		),
		review_count = (
			SELECT COUNT(*)
			FROM reviews
			WHERE dish_id = $1
		)
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID)

	if err != nil {
		return err
	}

	// Mirror stats in Redis for quick lookups.
	dishKey := fmt.Sprintf("dish:%d:%d", restaurantID, dishID)

	// Reload latest values from the database.
	var avgRating float64
	var reviewCount int
	err = db.QueryRow(`
		SELECT COALESCE(avg_rating, 0), COALESCE(review_count, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&avgRating, &reviewCount)

	if err != nil {
		return err
	}

	// Cache the refreshed snapshot.
	rdb.HSet(ctx, dishKey, map[string]interface{}{
		"avg_rating":  avgRating,
		"review_count": reviewCount,
		"last_updated": time.Now().Unix(),
	})

	rdb.Expire(ctx, dishKey, 24*time.Hour)

	return nil
}

func updateAnalytics(dishID, restaurantID int) error {
	// Update daily popularity scores.
	today := time.Now().Format("2006-01-02")
	dailyKey := fmt.Sprintf("analytics:daily:%s:%d", today, restaurantID)

	// Increment dish popularity for today.
	rdb.ZIncrBy(ctx, dailyKey, 1, strconv.Itoa(dishID))
	rdb.Expire(ctx, dailyKey, 7*24*time.Hour) // Keep for a week

	// Update the all-time leaderboard.
	allTimeKey := fmt.Sprintf("analytics:alltime:%d", restaurantID)

	// Pull the latest average rating.
	var avgRating float64
	err := db.QueryRow(`
		SELECT COALESCE(avg_rating, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&avgRating)

	if err != nil {
		return err
	}

	// Store the rating score in the sorted set.
	rdb.ZAdd(ctx, allTimeKey, redis.Z{
		Score:  avgRating,
		Member: strconv.Itoa(dishID),
	})

	return nil
}

func processReviewMessage(msg KafkaMessage) {
	log.Printf("Processing review: DishID=%d, RestaurantID=%d, Rating=%d",
		msg.DishID, msg.RestaurantID, msg.Rating)

	// Update dish rating
	if err := updateDishRating(msg.DishID, msg.RestaurantID, msg.Rating); err != nil {
		log.Printf("Error updating dish rating: %v", err)
		return
	}

	// Update analytics
	if err := updateAnalytics(msg.DishID, msg.RestaurantID); err != nil {
		log.Printf("Error updating analytics: %v", err)
		return
	}

	log.Printf("Successfully processed review for dish %d", msg.DishID)
}

func startConsumer() {
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   "reviews",
		GroupID: "agg-svc-consumer",
	})
	defer reader.Close()

	log.Println("Starting Aggregation Service consumer...")
	for {
		message, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Error reading message: %v", err)
			continue
		}

		var msg KafkaMessage
		if err := json.Unmarshal(message.Value, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		if msg.Type == "new_review" {
			processReviewMessage(msg)
		}
	}
}

func main() {
	initDB()
	defer db.Close()

	initRedis()
	defer rdb.Close()

	startConsumer()
}