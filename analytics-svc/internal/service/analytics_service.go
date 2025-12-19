package service

import (
	"context"
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"overcooked-simplified/analytics-svc/internal/domain"

	"github.com/redis/go-redis/v9"
)

type AnalyticsService struct {
	db  *sql.DB
	rdb *redis.Client
	ctx context.Context
}

func NewAnalyticsService(db *sql.DB, rdb *redis.Client) *AnalyticsService {
	return &AnalyticsService{
		db:  db,
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (s *AnalyticsService) TopToday() ([]domain.DishAnalytics, error) {
	today := time.Now().Format("2006-01-02")
	pattern := "analytics:daily:" + today + ":*"
	keys, err := s.rdb.Keys(s.ctx, pattern).Result()

	if err != nil || len(keys) == 0 {
		return s.topTodayFromDB()
	}

	var all []domain.DishAnalytics
	for _, key := range keys {
		result, err := s.rdb.ZRevRangeWithScores(s.ctx, key, 0, 4).Result()
		if err != nil || len(result) == 0 {
			continue
		}
		for _, member := range result {
			dishID, _ := strconv.Atoi(member.Member.(string))
			var dishName string
			var restID int
			if err := s.db.QueryRow("SELECT name, restaurant_id FROM dishes WHERE id = $1", dishID).Scan(&dishName, &restID); err != nil {
				continue
			}
			all = append(all, domain.DishAnalytics{
				DishID:       dishID,
				DishName:     dishName,
				RestaurantID: restID,
				Score:        member.Score,
			})
		}
	}

	if len(all) == 0 {
		return s.topTodayFromDB()
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > 10 {
		all = all[:10]
	}
	return all, nil
}

func (s *AnalyticsService) topTodayFromDB() ([]domain.DishAnalytics, error) {
	rows, err := s.db.Query(`
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
		return nil, err
	}
	defer rows.Close()

	var dishes []domain.DishAnalytics
	for rows.Next() {
		var d domain.DishAnalytics
		if err := rows.Scan(&d.DishID, &d.DishName, &d.RestaurantID, &d.Score); err != nil {
			continue
		}
		dishes = append(dishes, d)
	}
	return dishes, nil
}

func (s *AnalyticsService) TopAllTime() ([]domain.DishAnalytics, error) {
	keys, err := s.rdb.Keys(s.ctx, "analytics:alltime:*").Result()
	if err != nil || len(keys) == 0 {
		return s.topAllTimeFromDB()
	}

	var all []domain.DishAnalytics
	for _, key := range keys {
		result, err := s.rdb.ZRevRangeWithScores(s.ctx, key, 0, 9).Result()
		if err != nil {
			continue
		}
		for _, r := range result {
			dishID, _ := strconv.Atoi(r.Member.(string))
			var dishName string
			var restID, reviewCount int

			if err := s.db.QueryRow("SELECT name, restaurant_id, COALESCE(review_count,0) FROM dishes WHERE id = $1", dishID).
				Scan(&dishName, &restID, &reviewCount); err != nil {
				continue
			}

			all = append(all, domain.DishAnalytics{
				DishID:       dishID,
				DishName:     dishName,
				RestaurantID: restID,
				Score:        r.Score,
				ReviewCount:  reviewCount,
			})
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > 10 {
		all = all[:10]
	}
	return all, nil
}

func (s *AnalyticsService) topAllTimeFromDB() ([]domain.DishAnalytics, error) {
	rows, err := s.db.Query(`
		SELECT id, name, restaurant_id, COALESCE(avg_rating, 0) as score, review_count
		FROM dishes
		WHERE avg_rating > 0
		ORDER BY avg_rating DESC
		LIMIT 10
	`)
	if err != nil {
		return []domain.DishAnalytics{}, nil
	}
	defer rows.Close()

	var dishes []domain.DishAnalytics
	for rows.Next() {
		var d domain.DishAnalytics
		rows.Scan(&d.DishID, &d.DishName, &d.RestaurantID, &d.Score, &d.ReviewCount)
		dishes = append(dishes, d)
	}
	return dishes, nil
}

func (s *AnalyticsService) MostPopularDish(restaurantID int, date string) (*domain.DishAnalytics, error) {
	dailyKey := "analytics:daily:" + date + ":" + strconv.Itoa(restaurantID)
	result, err := s.rdb.ZRevRangeWithScores(s.ctx, dailyKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		return nil, nil
	}
	dishID, _ := strconv.Atoi(result[0].Member.(string))
	var dishName string
	if err := s.db.QueryRow("SELECT name FROM dishes WHERE id = $1", dishID).Scan(&dishName); err != nil {
		return nil, nil
	}
	return &domain.DishAnalytics{
		DishID:       dishID,
		DishName:     dishName,
		RestaurantID: restaurantID,
		Score:        result[0].Score,
	}, nil
}

func (s *AnalyticsService) BestRatedDish(restaurantID int) (*domain.DishAnalytics, error) {
	allTimeKey := "analytics:alltime:" + strconv.Itoa(restaurantID)
	result, err := s.rdb.ZRevRangeWithScores(s.ctx, allTimeKey, 0, 0).Result()
	if err != nil || len(result) == 0 {
		return nil, nil
	}
	dishID, _ := strconv.Atoi(result[0].Member.(string))

	var dishName string
	var reviewCount int
	if err := s.db.QueryRow(`
		SELECT name, COALESCE(review_count, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&dishName, &reviewCount); err != nil {
		return nil, nil
	}

	return &domain.DishAnalytics{
		DishID:       dishID,
		DishName:     dishName,
		RestaurantID: restaurantID,
		Score:        result[0].Score,
		ReviewCount:  reviewCount,
	}, nil
}

func (s *AnalyticsService) AnalyticsForRestaurant(restaurantID int, period string) domain.AnalyticsResponse {
	response := domain.AnalyticsResponse{}
	switch period {
	case "today":
		today := time.Now().Format("2006-01-02")
		if popular, _ := s.MostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularToday = popular
		}
	case "day":
		today := time.Now().Format("2006-01-02")
		if popular, _ := s.MostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularDish = popular
		}
	case "all":
		if best, _ := s.BestRatedDish(restaurantID); best != nil {
			response.BestRatedDish = best
		}
	default:
		today := time.Now().Format("2006-01-02")
		if popular, _ := s.MostPopularDish(restaurantID, today); popular != nil {
			response.MostPopularToday = popular
		}
		if best, _ := s.BestRatedDish(restaurantID); best != nil {
			response.BestRatedDish = best
		}
	}
	return response
}

func (s *AnalyticsService) DishStats(restaurantID, dishID int) (map[string]interface{}, error) {
	dishKey := "dish:" + strconv.Itoa(restaurantID) + ":" + strconv.Itoa(dishID)
	stats, err := s.rdb.HGetAll(s.ctx, dishKey).Result()
	if err != nil || len(stats) == 0 {
		return nil, err
	}
	avgRating, _ := strconv.ParseFloat(stats["avg_rating"], 64)
	reviewCount, _ := strconv.Atoi(stats["review_count"])
	return map[string]interface{}{
		"dish_id":      dishID,
		"avg_rating":   avgRating,
		"review_count": reviewCount,
		"last_updated": stats["last_updated"],
	}, nil
}

func (s *AnalyticsService) TopDishes(restaurantID, limit int) ([]domain.DishAnalytics, error) {
	allTimeKey := "analytics:alltime:" + strconv.Itoa(restaurantID)
	results, err := s.rdb.ZRevRangeWithScores(s.ctx, allTimeKey, 0, int64(limit-1)).Result()
	if err != nil {
		return []domain.DishAnalytics{}, nil
	}

	var top []domain.DishAnalytics
	for _, result := range results {
		dishID, _ := strconv.Atoi(result.Member.(string))
		var dishName string
		if err := s.db.QueryRow("SELECT name FROM dishes WHERE id = $1", dishID).Scan(&dishName); err != nil {
			continue
		}
		top = append(top, domain.DishAnalytics{
			DishID:       dishID,
			DishName:     dishName,
			RestaurantID: restaurantID,
			Score:        result.Score,
		})
	}
	return top, nil
}

func (s *AnalyticsService) RatingDistribution(restaurantID int) (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		WHERE restaurant_id = $1
		GROUP BY rating
		ORDER BY rating
	`, restaurantID)
	if err != nil {
		return map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}, nil
	}
	defer rows.Close()

	distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	for rows.Next() {
		var rating, count int
		rows.Scan(&rating, &count)
		distribution[strconv.Itoa(rating)] = count
	}
	return distribution, nil
}

func (s *AnalyticsService) GlobalDistribution() (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		GROUP BY rating
		ORDER BY rating
	`)
	if err != nil {
		return map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}, nil
	}
	defer rows.Close()

	distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	for rows.Next() {
		var rating, count int
		rows.Scan(&rating, &count)
		distribution[strconv.Itoa(rating)] = count
	}
	return distribution, nil
}

func (s *AnalyticsService) extractRestaurantIDFromKey(key string) int {
	parts := strings.Split(key, ":")
	if len(parts) >= 5 {
		id, _ := strconv.Atoi(parts[4])
		return id
	}
	return 0
}
