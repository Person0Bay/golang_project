package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"overcooked-simplified/rate-svc/internal/domain"
	"overcooked-simplified/rate-svc/internal/mocks"
	"overcooked-simplified/rate-svc/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReviewService_CreateOrUpdate(t *testing.T) {
	repository := mocks.NewReviewRepository(t)
	cache := mocks.NewReviewCache(t)
	publisher := mocks.NewReviewPublisher(t)

	svc := service.NewReviewService(repository, cache, publisher)

	ctx := context.Background()

	tests := []struct {
		name          string
		review        *domain.Review
		prepareMocks  func()
		expectedError error
	}{
		{
			name: "success_create_new_review",
			review: &domain.Review{
				DishID: 1, OrderID: 99, RestaurantID: 10, Rating: 5, Comment: "Great!",
			},
			prepareMocks: func() {
				repository.On("ValidateDishInOrder", 1, 99, 10).Return(true, nil).Once()
				cache.On("ReviewMarkerKey", 1, 99).Return("review:1:99").Once()
				cache.On("Exists", ctx, "review:1:99").Return(false, nil).Once()
				repository.On("GetExistingReviewID", 1, 99, 10).Return(0, errors.New("not found")).Once()
				repository.On("InsertReview", mock.Anything).Return(nil).Once()
				cache.On("SetMarker", ctx, "review:1:99").Return(nil).Once()
				publisher.On("PublishReview", ctx, mock.Anything).Return(nil).Once()
			},
			expectedError: nil,
		},
		{
			name: "error_dish_not_in_order",
			review: &domain.Review{
				DishID: 2, OrderID: 99, RestaurantID: 10, Rating: 3,
			},
			prepareMocks: func() {
				repository.On("ValidateDishInOrder", 2, 99, 10).Return(false, nil).Once()
			},
			expectedError: service.ErrDishNotInOrder,
		},
		{
			name: "error_duplicate_review",
			review: &domain.Review{
				DishID: 3, OrderID: 99, RestaurantID: 10, Rating: 4,
			},
			prepareMocks: func() {
				repository.On("ValidateDishInOrder", 3, 99, 10).Return(true, nil).Once()
				cache.On("ReviewMarkerKey", 3, 99).Return("review:3:99").Once()
				cache.On("Exists", ctx, "review:3:99").Return(true, nil).Once()
			},
			expectedError: service.ErrDuplicateReview,
		},
		{
			name: "success_update_existing_review",
			review: &domain.Review{
				DishID: 4, OrderID: 99, RestaurantID: 10, Rating: 5, Comment: "Updated",
			},
			prepareMocks: func() {
				repository.On("ValidateDishInOrder", 4, 99, 10).Return(true, nil).Once()
				cache.On("ReviewMarkerKey", 4, 99).Return("review:4:99").Once()
				cache.On("Exists", ctx, "review:4:99").Return(false, nil).Once()
				repository.On("GetExistingReviewID", 4, 99, 10).Return(42, nil).Once()
				repository.On("UpdateReview", 42, mock.Anything).Return(nil).Once()
				cache.On("SetMarker", ctx, "review:4:99").Return(nil).Once()
				publisher.On("PublishReview", ctx, mock.Anything).Return(nil).Once()
			},
			expectedError: nil,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.prepareMocks()
			err := svc.CreateOrUpdate(ctx, testCase.review)
			assert.ErrorIs(t, err, testCase.expectedError)
		})
	}
}

func TestReviewService_ListDishReviews(t *testing.T) {
	repository := mocks.NewReviewRepository(t)
	cache := mocks.NewReviewCache(t)
	publisher := mocks.NewReviewPublisher(t)

	svc := service.NewReviewService(repository, cache, publisher)

	expectedReviews := []domain.Review{
		{ID: 1, DishID: 1, OrderID: 99, RestaurantID: 10, Rating: 5, CreatedAt: time.Now()},
		{ID: 2, DishID: 1, OrderID: 100, RestaurantID: 10, Rating: 4, CreatedAt: time.Now()},
	}

	repository.On("ListDishReviews", 1, 10).Return(expectedReviews, nil).Once()

	reviews, err := svc.ListDishReviews(1, 10)
	assert.NoError(t, err)
	assert.Len(t, reviews, 2)
	assert.Equal(t, expectedReviews, reviews)
}
