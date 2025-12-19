package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"overcooked-simplified/rate-svc/internal/domain"
)

var (
	ErrDishNotInOrder  = errors.New("dish was not ordered for this check")
	ErrDuplicateReview = errors.New("review already exists for this dish and check")
)

type ReviewService struct {
	repository ReviewRepository
	cache      ReviewCache
	publisher  ReviewPublisher
}

func NewReviewService(repository ReviewRepository, cache ReviewCache, publisher ReviewPublisher) *ReviewService {
	return &ReviewService{
		repository: repository,
		cache:      cache,
		publisher:  publisher,
	}
}

func (s *ReviewService) CreateOrUpdate(ctx context.Context, review *domain.Review) error {
	valid, err := s.repository.ValidateDishInOrder(review.DishID, review.OrderID, review.RestaurantID)
	if err != nil {
		return fmt.Errorf("failed to validate order: %w", err)
	}
	if !valid {
		return ErrDishNotInOrder
	}

	cacheKey := s.cache.ReviewMarkerKey(review.DishID, review.OrderID)
	if exists, _ := s.cache.Exists(ctx, cacheKey); exists {
		return ErrDuplicateReview
	}

	existingID, err := s.repository.GetExistingReviewID(review.DishID, review.OrderID, review.RestaurantID)
	isUpdate := err == nil && existingID > 0
	if isUpdate {
		if err := s.repository.UpdateReview(existingID, review); err != nil {
			return err
		}
		review.ID = existingID
	} else if err := s.repository.InsertReview(review); err != nil {
		return err
	}

	_ = s.cache.SetMarker(ctx, cacheKey)

	if s.publisher != nil {
		_ = s.publisher.PublishReview(ctx, domain.KafkaMessage{
			Type:         "new_review",
			DishID:       review.DishID,
			RestaurantID: review.RestaurantID,
			OrderID:      review.OrderID,
			Rating:       review.Rating,
			Timestamp:    time.Now(),
		})
	}

	return nil
}

func (s *ReviewService) ListDishReviews(dishID, restaurantID int) ([]domain.Review, error) {
	return s.repository.ListDishReviews(dishID, restaurantID)
}
