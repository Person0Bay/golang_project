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
	// 1. Validate that the dish is actually part of the order/check
	valid, err := s.repository.ValidateDishInOrder(review.DishID, review.OrderID, review.RestaurantID)
	if err != nil {
		return fmt.Errorf("failed to validate order: %w", err)
	}
	if !valid {
		return ErrDishNotInOrder
	}

	// FIX: Removed the blocking Redis check here.
	// Previously, checking s.cache.Exists would return ErrDuplicateReview immediately,
	// preventing the code from reaching the Update logic below.

	// 2. Check Database to see if this is an Insert or an Update
	existingID, err := s.repository.GetExistingReviewID(review.DishID, review.OrderID, review.RestaurantID)

	// If err is nil and ID > 0, it means the review exists in DB
	isUpdate := err == nil && existingID > 0

	if isUpdate {
		// UPDATE PATH
		if err := s.repository.UpdateReview(existingID, review); err != nil {
			return err
		}
		// Set the ID to the existing one so the response is correct
		review.ID = existingID
	} else {
		// INSERT PATH
		if err := s.repository.InsertReview(review); err != nil {
			return err
		}
	}

	// 3. Update/Refresh the Cache Marker
	// We set this regardless of update/insert to keep the cache warm
	cacheKey := s.cache.ReviewMarkerKey(review.DishID, review.OrderID)
	_ = s.cache.SetMarker(ctx, cacheKey)

	// 4. Publish Event
	if s.publisher != nil {
		// Determine event type based on operation
		eventType := "new_review"
		if isUpdate {
			eventType = "updated_review"
		}

		_ = s.publisher.PublishReview(ctx, domain.KafkaMessage{
			Type:         eventType,
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
