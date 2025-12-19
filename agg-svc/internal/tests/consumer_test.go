package tests

import (
	"errors"
	"testing"

	"overcooked-simplified/agg-svc/internal/domain"
	"overcooked-simplified/agg-svc/internal/mocks"
	"overcooked-simplified/agg-svc/internal/service"
)

func TestConsumer_ProcessReview(t *testing.T) {
	tests := []struct {
		name           string
		inputMessage   domain.KafkaMessage
		setupMockStore func(*mocks.StoreInterface)
	}{
		{
			name: "success",
			inputMessage: domain.KafkaMessage{
				Type:         "new_review",
				DishID:       1,
				RestaurantID: 10,
				Rating:       5,
			},
			setupMockStore: func(mockStore *mocks.StoreInterface) {
				mockStore.On("UpdateDishRating", 1, 10).Return(nil)
				mockStore.On("UpdateAnalytics", 1, 10).Return(nil)
			},
		},
		{
			name: "UpdateDishRating error",
			inputMessage: domain.KafkaMessage{
				Type:         "new_review",
				DishID:       1,
				RestaurantID: 10,
				Rating:       5,
			},
			setupMockStore: func(mockStore *mocks.StoreInterface) {
				mockStore.On("UpdateDishRating", 1, 10).Return(errors.New("db connection failed"))
			},
		},
		{
			name: "UpdateAnalytics error",
			inputMessage: domain.KafkaMessage{
				Type:         "new_review",
				DishID:       1,
				RestaurantID: 10,
				Rating:       5,
			},
			setupMockStore: func(mockStore *mocks.StoreInterface) {
				mockStore.On("UpdateDishRating", 1, 10).Return(nil)
				mockStore.On("UpdateAnalytics", 1, 10).Return(errors.New("redis error"))
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockStore := mocks.NewStoreInterface(t)
			testCase.setupMockStore(mockStore)

			consumer := &service.Consumer{
				Store: mockStore,
			}

			consumer.ProcessReview(testCase.inputMessage)
			mockStore.AssertExpectations(t)
		})
	}
}

func TestConsumer_InvalidMessageType(t *testing.T) {
	mockStore := mocks.NewStoreInterface(t)
	consumer := &service.Consumer{
		Store: mockStore,
	}

	message := domain.KafkaMessage{
		Type:         "unknown_type",
		DishID:       1,
		RestaurantID: 10,
		Rating:       5,
	}

	consumer.ProcessReview(message)
	mockStore.AssertNotCalled(t, "UpdateDishRating")
	mockStore.AssertNotCalled(t, "UpdateAnalytics")
}
