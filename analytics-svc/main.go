package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
)

type AnalyticsRequest struct {
	RestaurantID int    `json:"restaurant_id"`
	Period       string `json:"period"`
}

type AnalyticsResponse struct {
	MostPopularDish  *DishAnalytics `json:"most_popular_dish,omitempty"`
	BestRatedDish    *DishAnalytics `json:"best_rated_dish,omitempty"`
	MostPopularToday *DishAnalytics `json:"most_popular_today,omitempty"`
}

type DishAnalytics struct {
	DishID       int     `json:"dish_id"`
	DishName     string  `json:"dish_name"`
	RestaurantID int     `json:"restaurant_id"`
	Score        float64 `json:"score"`
	ReviewCount  int     `json:"review_count"`
}

var (
	rdb *redis.Client
	db  *sql.DB
	ctx = context.Background()
)

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
}

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")

	if dbHost == "" || dbPort == "" || dbName == "" || dbUser == "" {
		log.Fatal("Database environment variables are not fully configured")
	}

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

// getTopToday returns the top dishes for the current date (up to 10).
func getTopToday(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	pattern := fmt.Sprintf("analytics:daily:%s:*", today)
	keys, err := rdb.Keys(ctx, pattern).Result()

	// Fallback: when Redis has no entries, derive stats from order history.
	if err != nil || len(keys) == 0 {
		rows, err := db.Query(`
			SELECT d.id, d.name, d.restaurant_id, COUNT(oi.id) as score
			FROM dishes d
			JOIN order_items oi ON d.id = oi.dish_id
			JOIN orders o ON oi.order_id = o.id
			WHERE o.created_at::date = CURRENT_DATE
			GROUP BY d.id, d.name, d.restaurant_id
			ORDER BY score DESC
			LIMIT 10
		`)
		if err != nil {
			log.Printf("Error querying database for top today: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]DishAnalytics{})
			return
		}
		defer rows.Close()

		var dishes []DishAnalytics
		for rows.Next() {
			var d DishAnalytics
			if err := rows.Scan(&d.DishID, &d.DishName, &d.RestaurantID, &d.Score); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			dishes = append(dishes, d)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dishes)
		return
	}

	// Redis path: collect ALL popular dishes from all restaurants
	var allDishes []DishAnalytics
	for _, key := range keys {
		// Get top 5 dishes from each restaurant (enough to cover potential top 10 globally)
		result, err := rdb.ZRevRangeWithScores(ctx, key, 0, 4).Result()
		if err != nil || len(result) == 0 {
			continue
		}

		for _, member := range result {
			dishID, _ := strconv.Atoi(member.Member.(string))

			var dishName string
			var restID int
			err = db.QueryRow("SELECT name, restaurant_id FROM dishes WHERE id = $1", dishID).Scan(&dishName, &restID)
			if err != nil {
				// Dish was removed, skip it
				continue
			}

			allDishes = append(allDishes, DishAnalytics{
				DishID:       dishID,
				DishName:     dishName,
				RestaurantID: restID,
				Score:        member.Score,
			})
		}
	}

	// If Redis didn't provide enough data, fallback to database
	if len(allDishes) == 0 {
		rows, err := db.Query(`
			SELECT d.id, d.name, d.restaurant_id, COUNT(oi.id) as score
			FROM dishes d
			JOIN order_items oi ON d.id = oi.dish_id
			JOIN orders o ON oi.order_id = o.id
			WHERE o.created_at::date = CURRENT_DATE
			GROUP BY d.id, d.name, d.restaurant_id
			ORDER BY score DESC
			LIMIT 10
		`)
		if err != nil {
			log.Printf("Error querying database after Redis fallback: %v", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]DishAnalytics{})
			return
		}
		defer rows.Close()

		var dishes []DishAnalytics
		for rows.Next() {
			var d DishAnalytics
			if err := rows.Scan(&d.DishID, &d.DishName, &d.RestaurantID, &d.Score); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
			}
			dishes = append(dishes, d)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dishes)
		return
	}

	// Sort all dishes by score (descending)
	sort.Slice(allDishes, func(i, j int) bool {
		return allDishes[i].Score > allDishes[j].Score
	})

	// Limit to top 10 globally
	if len(allDishes) > 10 {
		allDishes = allDishes[:10]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allDishes)
}

// getTopAllTime returns the best dishes across the full dataset.
func getTopAllTime(w http.ResponseWriter, r *http.Request) {
	pattern := "analytics:alltime:*"
	keys, err := rdb.Keys(ctx, pattern).Result()

	if err != nil || len(keys) == 0 {
		rows, err := db.Query(`
			SELECT id, name, restaurant_id, COALESCE(avg_rating, 0) as score, review_count
			FROM dishes
			WHERE avg_rating > 0
			ORDER BY avg_rating DESC
			LIMIT 10
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]DishAnalytics{})
			return
		}
		defer rows.Close()

		var dishes []DishAnalytics
		for rows.Next() {
			var d DishAnalytics
			rows.Scan(&d.DishID, &d.DishName, &d.RestaurantID, &d.Score, &d.ReviewCount)
			dishes = append(dishes, d)
		}

		w.Header().Set("Content-Type", "application/json")
		if dishes == nil {
			json.NewEncoder(w).Encode([]DishAnalytics{})
		} else {
			json.NewEncoder(w).Encode(dishes)
		}
		return
	}

	// Redis-backed path for cached historical analytics.
	var allDishes []DishAnalytics
	for _, key := range keys {
		result, err := rdb.ZRevRangeWithScores(ctx, key, 0, 9).Result()
		if err != nil {
			continue
		}

		for _, r := range result {
			dishID, _ := strconv.Atoi(r.Member.(string))

			var dishName string
			var restID int

			err := db.QueryRow("SELECT name, restaurant_id FROM dishes WHERE id = $1", dishID).Scan(&dishName, &restID)

			if err != nil {
				// Dish was removed, skip it
				continue
			}

			var reviewCount int
			db.QueryRow("SELECT review_count FROM dishes WHERE id = $1", dishID).Scan(&reviewCount)

			allDishes = append(allDishes, DishAnalytics{
				DishID:       dishID,
				DishName:     dishName,
				RestaurantID: restID,
				Score:        r.Score,
				ReviewCount:  reviewCount,
			})
		}
	}

	for i := 0; i < len(allDishes); i++ {
		for j := i + 1; j < len(allDishes); j++ {
			if allDishes[j].Score > allDishes[i].Score {
				allDishes[i], allDishes[j] = allDishes[j], allDishes[i]
			}
		}
	}
	if len(allDishes) > 10 {
		allDishes = allDishes[:10]
	}

	w.Header().Set("Content-Type", "application/json")
	if allDishes == nil {
		json.NewEncoder(w).Encode([]DishAnalytics{})
	} else {
		json.NewEncoder(w).Encode(allDishes)
	}
}

func extractRestaurantIDFromKey(key string) int {
	parts := strings.Split(key, ":")
	if len(parts) >= 5 {
		id, _ := strconv.Atoi(parts[4])
		return id
	}
	return 0
}

func extractRestaurantIDFromAllTimeKey(key string) int {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 {
		id, _ := strconv.Atoi(parts[2])
		return id
	}
	return 0
}

func getMostPopularDish(restaurantID int, date string) (*DishAnalytics, error) {
	dailyKey := fmt.Sprintf("analytics:daily:%s:%d", date, restaurantID)

	result, err := rdb.ZRevRangeWithScores(ctx, dailyKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		return nil, nil
	}

	dishID, _ := strconv.Atoi(result[0].Member.(string))

	var dishName string
	err = db.QueryRow("SELECT name FROM dishes WHERE id = $1", dishID).Scan(&dishName)
	if err != nil {
		return nil, nil
	}

	return &DishAnalytics{
		DishID:       dishID,
		DishName:     dishName,
		RestaurantID: restaurantID,
		Score:        result[0].Score,
	}, nil
}

// getBestRatedDish is defined as a variable so that tests can swap in a mock
// implementation. The default implementation queries Redis and Postgres.
var getBestRatedDish = func(restaurantID int) (*DishAnalytics, error) {
	allTimeKey := fmt.Sprintf("analytics:alltime:%d", restaurantID)

	result, err := rdb.ZRevRangeWithScores(ctx, allTimeKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		return nil, nil
	}

	dishID, _ := strconv.Atoi(result[0].Member.(string))

	var dishName string
	var reviewCount int

	err = db.QueryRow(`
		SELECT name, COALESCE(review_count, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&dishName, &reviewCount)

	if err != nil {
		return nil, nil
	}

	return &DishAnalytics{
		DishID:       dishID,
		DishName:     dishName,
		RestaurantID: restaurantID,
		Score:        result[0].Score,
		ReviewCount:  reviewCount,
	}, nil
}

func getAnalytics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}

	response := AnalyticsResponse{}

	switch period {
	case "today":
		today := time.Now().Format("2006-01-02")
		if popular, _ := getMostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularToday = popular
		}

	case "day":
		today := time.Now().Format("2006-01-02")
		if popular, _ := getMostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularDish = popular
		}

	case "all":
		if best, _ := getBestRatedDish(restaurantID); best != nil {
			response.BestRatedDish = best
		}

	default:
		today := time.Now().Format("2006-01-02")
		if popular, _ := getMostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularToday = popular
		}
		if best, _ := getBestRatedDish(restaurantID); best != nil {
			response.BestRatedDish = best
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getDishStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])
	dishID, _ := strconv.Atoi(vars["dishId"])

	dishKey := fmt.Sprintf("dish:%d:%d", restaurantID, dishID)

	stats, err := rdb.HGetAll(ctx, dishKey).Result()
	if err != nil || len(stats) == 0 {
		http.Error(w, "Dish stats not found", http.StatusNotFound)
		return
	}

	avgRating, _ := strconv.ParseFloat(stats["avg_rating"], 64)
	reviewCount, _ := strconv.Atoi(stats["review_count"])

	response := map[string]interface{}{
		"dish_id":      dishID,
		"avg_rating":   avgRating,
		"review_count": reviewCount,
		"last_updated": stats["last_updated"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getTopDishes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "10"
	}
	limit, _ := strconv.Atoi(limitStr)

	allTimeKey := fmt.Sprintf("analytics:alltime:%d", restaurantID)

	results, err := rdb.ZRevRangeWithScores(ctx, allTimeKey, 0, int64(limit-1)).Result()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]DishAnalytics{})
		return
	}

	var topDishes []DishAnalytics
	for _, result := range results {
		dishID, _ := strconv.Atoi(result.Member.(string))

		var dishName string
		err := db.QueryRow("SELECT name FROM dishes WHERE id = $1", dishID).Scan(&dishName)
		if err != nil {
			continue // Dish was removed
		}

		topDishes = append(topDishes, DishAnalytics{
			DishID:       dishID,
			DishName:     dishName,
			RestaurantID: restaurantID,
			Score:        result.Score,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if topDishes == nil {
		json.NewEncoder(w).Encode([]DishAnalytics{})
	} else {
		json.NewEncoder(w).Encode(topDishes)
	}
}

// GET /api/restaurants/{restaurantId}/analytics/rating-distribution
func getRatingDistribution(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	rows, err := db.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		WHERE restaurant_id = $1
		GROUP BY rating
		ORDER BY rating
	`, restaurantID)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0})
		return
	}
	defer rows.Close()

	distribution := map[string]int{
		"1": 0, "2": 0, "3": 0, "4": 0, "5": 0,
	}

	for rows.Next() {
		var rating int
		var count int
		rows.Scan(&rating, &count)
		distribution[strconv.Itoa(rating)] = count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(distribution)
}

// GET /api/analytics/rating-distribution - Global distribution
func getGlobalRatingDistribution(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		GROUP BY rating
		ORDER BY rating
	`)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0})
		return
	}
	defer rows.Close()

	distribution := map[string]int{
		"1": 0, "2": 0, "3": 0, "4": 0, "5": 0,
	}

	for rows.Next() {
		var rating int
		var count int
		rows.Scan(&rating, &count)
		distribution[strconv.Itoa(rating)] = count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(distribution)
}

func main() {
	initDB()
	defer db.Close()

	initRedis()
	defer rdb.Close()

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")
	r.HandleFunc("/api/analytics/top-today", getTopToday).Methods("GET")
	r.HandleFunc("/api/analytics/top-alltime", getTopAllTime).Methods("GET")

	r.HandleFunc("/api/restaurants/{restaurantId}/analytics", getAnalytics).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/stats", getDishStats).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/top-dishes", getTopDishes).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/analytics/rating-distribution", getRatingDistribution).Methods("GET")
	r.HandleFunc("/api/analytics/rating-distribution", getGlobalRatingDistribution).Methods("GET")
	handler := cors.Default().Handler(r)

	log.Println("Analytics Service starting on port 8083")
	log.Fatal(http.ListenAndServe(":8083", handler))
}
