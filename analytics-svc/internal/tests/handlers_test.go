package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	httpapi "overcooked-simplified/analytics-svc/internal/api/http"
	"overcooked-simplified/analytics-svc/internal/domain"
	"overcooked-simplified/analytics-svc/internal/mocks"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetAnalyticsHandler(t *testing.T) {
	tests := []struct {
		name         string
		restaurantID string
		period       string
		mockResponse domain.AnalyticsResponse
		wantCode     int
	}{
		{
			name:         "success all period",
			restaurantID: "1",
			period:       "all",
			mockResponse: domain.AnalyticsResponse{
				BestRatedDish: &domain.DishAnalytics{DishID: 1, Score: 4.5},
			},
			wantCode: http.StatusOK,
		},
		{
			name:         "success today period",
			restaurantID: "2",
			period:       "today",
			mockResponse: domain.AnalyticsResponse{
				MostPopularToday: &domain.DishAnalytics{DishID: 2, Score: 10.0},
			},
			wantCode: http.StatusOK,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAnalytics := new(mocks.AnalyticsInterface)
			handler := httpapi.NewHandler(mockAnalytics)

			req := httptest.NewRequest(http.MethodGet, "/api/restaurants/"+testCase.restaurantID+"/analytics?period="+testCase.period, nil)
			req = mux.SetURLVars(req, map[string]string{"restaurantId": testCase.restaurantID})
			w := httptest.NewRecorder()

			mockAnalytics.On("AnalyticsForRestaurant", mock.AnythingOfType("int"), testCase.period).
				Return(testCase.mockResponse)

			r := mux.NewRouter()
			handler.RegisterRoutes(r)
			r.ServeHTTP(w, req)

			assert.Equal(t, testCase.wantCode, w.Code)
			mockAnalytics.AssertExpectations(t)
		})
	}
}

func TestGetDishStatsHandler(t *testing.T) {
	tests := []struct {
		name         string
		restaurantID string
		dishID       string
		mockStats    map[string]interface{}
		mockError    error
		wantCode     int
	}{
		{
			name:         "success",
			restaurantID: "1",
			dishID:       "1",
			mockStats: map[string]interface{}{
				"dish_id":      1,
				"avg_rating":   4.5,
				"review_count": 10,
			},
			wantCode: http.StatusOK,
		},
		{
			name:         "not found",
			restaurantID: "1",
			dishID:       "999",
			mockError:    errors.New("not found"),
			wantCode:     http.StatusNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAnalytics := new(mocks.AnalyticsInterface)
			handler := httpapi.NewHandler(mockAnalytics)

			restID, _ := strconv.Atoi(testCase.restaurantID)
			dishID, _ := strconv.Atoi(testCase.dishID)

			req := httptest.NewRequest(http.MethodGet, "/api/restaurants/"+testCase.restaurantID+"/dishes/"+testCase.dishID+"/stats", nil)
			req = mux.SetURLVars(req, map[string]string{
				"restaurantId": testCase.restaurantID,
				"dishId":       testCase.dishID,
			})
			w := httptest.NewRecorder()

			mockAnalytics.On("DishStats", restID, dishID).Return(testCase.mockStats, testCase.mockError)

			r := mux.NewRouter()
			handler.RegisterRoutes(r)
			r.ServeHTTP(w, req)

			assert.Equal(t, testCase.wantCode, w.Code)
			mockAnalytics.AssertExpectations(t)
		})
	}
}

func TestGetTopTodayHandler(t *testing.T) {
	mockAnalytics := new(mocks.AnalyticsInterface)
	handler := httpapi.NewHandler(mockAnalytics)

	mockAnalytics.On("TopToday").Return([]domain.DishAnalytics{
		{DishID: 1, DishName: "Pizza", Score: 5.0},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/top-today", nil)
	w := httptest.NewRecorder()

	r := mux.NewRouter()
	handler.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockAnalytics.AssertExpectations(t)
}
