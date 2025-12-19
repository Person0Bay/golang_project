package storage

import (
	"database/sql"
	"fmt"

	"overcooked-simplified/rate-svc/internal/domain"
)

type PostgresRepository struct {
	DB *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{DB: db}
}

func (r *PostgresRepository) ValidateDishInOrder(dishID, orderID, restaurantID int) (bool, error) {
	var exists bool
	err := r.DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM order_items oi
			JOIN orders o ON oi.order_id = o.id
			WHERE oi.dish_id = $1 AND oi.order_id = $2 AND o.restaurant_id = $3
		)
	`, dishID, orderID, restaurantID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) GetExistingReviewID(dishID, orderID, restaurantID int) (int, error) {
	var id int
	err := r.DB.QueryRow(`
		SELECT id FROM reviews
		WHERE dish_id = $1 AND order_id = $2 AND restaurant_id = $3
	`, dishID, orderID, restaurantID).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *PostgresRepository) InsertReview(review *domain.Review) error {
	return r.DB.QueryRow(`
		INSERT INTO reviews (dish_id, order_id, restaurant_id, rating, comment)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, review.DishID, review.OrderID, review.RestaurantID, review.Rating, review.Comment).
		Scan(&review.ID, &review.CreatedAt)
}

func (r *PostgresRepository) UpdateReview(id int, review *domain.Review) error {
	_, err := r.DB.Exec(`
		UPDATE reviews
		SET rating = $1, comment = $2, created_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`, review.Rating, review.Comment, id)
	return err
}

func (r *PostgresRepository) ListDishReviews(dishID, restaurantID int) ([]domain.Review, error) {
	rows, err := r.DB.Query(`
		SELECT id, dish_id, order_id, restaurant_id, rating, comment, created_at
		FROM reviews
		WHERE dish_id = $1 AND restaurant_id = $2
		ORDER BY created_at DESC
	`, dishID, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var rev domain.Review
		if err := rows.Scan(&rev.ID, &rev.DishID, &rev.OrderID, &rev.RestaurantID, &rev.Rating, &rev.Comment, &rev.CreatedAt); err != nil {
			continue
		}
		reviews = append(reviews, rev)
	}
	return reviews, nil
}

func (r *PostgresRepository) RatingDistribution(restaurantID int) (map[string]int, error) {
	rows, err := r.DB.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		WHERE restaurant_id = $1
		GROUP BY rating
		ORDER BY rating
	`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			continue
		}
		distribution[fmt.Sprintf("%d", rating)] = count
	}
	return distribution, nil
}

func (r *PostgresRepository) GlobalDistribution() (map[string]int, error) {
	rows, err := r.DB.Query(`
		SELECT rating, COUNT(*) as count
		FROM reviews
		GROUP BY rating
		ORDER BY rating
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			continue
		}
		distribution[fmt.Sprintf("%d", rating)] = count
	}
	return distribution, nil
}
