package tests

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	httpapi "overcooked-simplified/dish-svc/internal/api/http"
	"overcooked-simplified/dish-svc/internal/domain"
	"overcooked-simplified/dish-svc/internal/mocks"
	"overcooked-simplified/dish-svc/internal/service"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateRestaurantHandler(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		setupMock func(*mocks.RestaurantRepository)
		wantCode  int
	}{
		{
			name: "valid request",
			body: `{"name":"Test","address":"Addr","description":"Desc"}`,
			setupMock: func(m *mocks.RestaurantRepository) {
				m.On("CreateRestaurant", mock.AnythingOfType("*domain.Restaurant")).Return(nil).Once()
			},
			wantCode: http.StatusOK,
		},
		{
			name:      "invalid JSON",
			body:      `{invalid}`,
			setupMock: func(m *mocks.RestaurantRepository) {},
			wantCode:  http.StatusBadRequest,
		},
		{
			name: "database error",
			body: `{"name":"Test"}`,
			setupMock: func(m *mocks.RestaurantRepository) {
				m.On("CreateRestaurant", mock.AnythingOfType("*domain.Restaurant")).Return(errors.New("db error")).Once()
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := new(mocks.RestaurantRepository)
			restService := service.NewRestaurantService(mockRepo)
			handler := httpapi.NewHandler(restService, nil, nil)

			testCase.setupMock(mockRepo)

			req := httptest.NewRequest("POST", "/api/restaurants", bytes.NewBufferString(testCase.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r := mux.NewRouter()
			handler.RegisterRoutes(r)
			r.ServeHTTP(w, req)

			assert.Equal(t, testCase.wantCode, w.Code)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestGetRestaurantHandler(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		mockRest  *domain.Restaurant
		mockError error
		wantCode  int
	}{
		{
			name:     "found",
			id:       "1",
			mockRest: &domain.Restaurant{ID: 1, Name: "Test"},
			wantCode: http.StatusOK,
		},
		{
			name:      "not found",
			id:        "999",
			mockError: errors.New("not found"),
			wantCode:  http.StatusNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := new(mocks.RestaurantRepository)
			restService := service.NewRestaurantService(mockRepo)
			handler := httpapi.NewHandler(restService, nil, nil)

			if testCase.mockError != nil {
				mockRepo.On("GetRestaurant", mock.Anything).Return(nil, testCase.mockError).Once()
			} else {
				mockRepo.On("GetRestaurant", mock.Anything).Return(testCase.mockRest, nil).Once()
			}

			req := httptest.NewRequest("GET", "/api/restaurants/"+testCase.id, nil)
			req = mux.SetURLVars(req, map[string]string{"id": testCase.id})
			w := httptest.NewRecorder()

			r := mux.NewRouter()
			handler.RegisterRoutes(r)
			r.ServeHTTP(w, req)

			assert.Equal(t, testCase.wantCode, w.Code)
		})
	}
}
