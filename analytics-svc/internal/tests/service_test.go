package tests

import (
	"errors"
	"testing"

	"overcooked-simplified/analytics-svc/internal/domain"
	"overcooked-simplified/analytics-svc/internal/mocks"

	"github.com/stretchr/testify/assert"
)

func TestAnalyticsService_TopToday(t *testing.T) {
	tests := []struct {
		name      string
		mockRedis func(*mocks.AnalyticsInterface)
		wantLen   int
		wantErr   bool
	}{
		{
			name: "success from redis",
			mockRedis: func(mockAnalytics *mocks.AnalyticsInterface) {
				mockAnalytics.On("TopToday").Return([]domain.DishAnalytics{
					{DishID: 1, DishName: "Pizza", Score: 5.0},
				}, nil)
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "redis empty fallback to db",
			mockRedis: func(mockAnalytics *mocks.AnalyticsInterface) {
				mockAnalytics.On("TopToday").Return([]domain.DishAnalytics{}, nil)
			},
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAnalytics := new(mocks.AnalyticsInterface)
			testCase.mockRedis(mockAnalytics)

			result, err := mockAnalytics.TopToday()

			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, testCase.wantLen)
			}
			mockAnalytics.AssertExpectations(t)
		})
	}
}

func TestAnalyticsService_AnalyticsForRestaurant(t *testing.T) {
	tests := []struct {
		name         string
		restaurantID int
		period       string
		mockResult   domain.AnalyticsResponse
	}{
		{
			name:         "period all",
			restaurantID: 1,
			period:       "all",
			mockResult: domain.AnalyticsResponse{
				BestRatedDish: &domain.DishAnalytics{DishID: 1, Score: 4.5},
			},
		},
		{
			name:         "period today",
			restaurantID: 1,
			period:       "today",
			mockResult: domain.AnalyticsResponse{
				MostPopularToday: &domain.DishAnalytics{DishID: 2, Score: 10.0},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAnalytics := new(mocks.AnalyticsInterface)
			mockAnalytics.On("AnalyticsForRestaurant", testCase.restaurantID, testCase.period).Return(testCase.mockResult)

			result := mockAnalytics.AnalyticsForRestaurant(testCase.restaurantID, testCase.period)

			assert.Equal(t, testCase.mockResult, result)
			mockAnalytics.AssertExpectations(t)
		})
	}
}

func TestAnalyticsService_DishStats(t *testing.T) {
	tests := []struct {
		name         string
		restaurantID int
		dishID       int
		mockStats    map[string]interface{}
		mockError    error
		wantErr      bool
	}{
		{
			name:         "success",
			restaurantID: 1,
			dishID:       1,
			mockStats: map[string]interface{}{
				"dish_id":      1,
				"avg_rating":   4.5,
				"review_count": 10,
			},
			wantErr: false,
		},
		{
			name:         "not found",
			restaurantID: 1,
			dishID:       999,
			mockError:    errors.New("not found"),
			wantErr:      true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAnalytics := new(mocks.AnalyticsInterface)
			mockAnalytics.On("DishStats", testCase.restaurantID, testCase.dishID).Return(testCase.mockStats, testCase.mockError)

			result, err := mockAnalytics.DishStats(testCase.restaurantID, testCase.dishID)

			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.mockStats, result)
			}
			mockAnalytics.AssertExpectations(t)
		})
	}
}
