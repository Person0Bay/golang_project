package service

import (
	"context"
	"overcooked-simplified/rate-svc/internal/domain"
)

type ReviewServiceInterface interface {
	CreateOrUpdate(ctx context.Context, review *domain.Review) error
	ListDishReviews(dishID, restaurantID int) ([]domain.Review, error)
}

type ReviewRepository interface {
	ValidateDishInOrder(dishID, orderID, restaurantID int) (bool, error)
	GetExistingReviewID(dishID, orderID, restaurantID int) (int, error)
	InsertReview(review *domain.Review) error
	UpdateReview(id int, review *domain.Review) error
	ListDishReviews(dishID, restaurantID int) ([]domain.Review, error)
}

type ReviewCache interface {
	ReviewMarkerKey(dishID, orderID int) string
	Exists(ctx context.Context, key string) (bool, error)
	SetMarker(ctx context.Context, key string) error
}

type ReviewPublisher interface {
	PublishReview(ctx context.Context, msg domain.KafkaMessage) error
}

var _ ReviewServiceInterface = (*ReviewService)(nil)
