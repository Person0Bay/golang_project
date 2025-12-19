package service

import (
	"context"
	"overcooked-simplified/agg-svc/internal/domain"
	"overcooked-simplified/agg-svc/internal/storage"
)

type StoreInterface interface {
	UpdateDishRating(dishID, restaurantID int) error
	UpdateAnalytics(dishID, restaurantID int) error
}

type ConsumerInterface interface {
	Start(ctx context.Context)
	ProcessReview(msg domain.KafkaMessage)
}

var _ StoreInterface = (*storage.Store)(nil)
