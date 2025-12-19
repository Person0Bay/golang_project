package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	db  *sql.DB
	rdb *redis.Client
	ctx context.Context
}

func NewStore(db *sql.DB, rdb *redis.Client) *Store {
	return &Store{
		db:  db,
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (s *Store) UpdateDishRating(dishID, restaurantID int) error {
	_, err := s.db.Exec(`
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

	var avgRating float64
	var reviewCount int
	if err := s.db.QueryRow(`
		SELECT COALESCE(avg_rating, 0), COALESCE(review_count, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&avgRating, &reviewCount); err != nil {
		return err
	}

	key := fmt.Sprintf("dish:%d:%d", restaurantID, dishID)
	s.rdb.HSet(s.ctx, key, map[string]interface{}{
		"avg_rating":   avgRating,
		"review_count": reviewCount,
		"last_updated": time.Now().Unix(),
	})
	s.rdb.Expire(s.ctx, key, 24*time.Hour)
	return nil
}

func (s *Store) UpdateAnalytics(dishID, restaurantID int) error {
	today := time.Now().Format("2006-01-02")
	dailyKey := fmt.Sprintf("analytics:daily:%s:%d", today, restaurantID)
	s.rdb.ZIncrBy(s.ctx, dailyKey, 1, strconv.Itoa(dishID))
	s.rdb.Expire(s.ctx, dailyKey, 7*24*time.Hour)

	allTimeKey := fmt.Sprintf("analytics:alltime:%d", restaurantID)
	var avgRating float64
	if err := s.db.QueryRow(`
		SELECT COALESCE(avg_rating, 0)
		FROM dishes
		WHERE id = $1 AND restaurant_id = $2
	`, dishID, restaurantID).Scan(&avgRating); err != nil {
		return err
	}
	s.rdb.ZAdd(s.ctx, allTimeKey, redis.Z{
		Score:  avgRating,
		Member: strconv.Itoa(dishID),
	})
	return nil
}
