package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httpapi "overcooked-simplified/rate-svc/internal/api/http"
	"overcooked-simplified/rate-svc/internal/domain"
	"overcooked-simplified/rate-svc/internal/mocks"
	"overcooked-simplified/rate-svc/internal/service"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestRouter(mockSvc *mocks.ReviewServiceInterface) *mux.Router {
	handler := &httpapi.Handler{Reviews: mockSvc}
	r := mux.NewRouter()
	handler.RegisterRoutes(r)
	return r
}

func TestHandler_createReview(t *testing.T) {
	mockSvc := mocks.NewReviewServiceInterface(t)
	router := setupTestRouter(mockSvc)

	tests := []struct {
		name         string
		payload      string
		prepareMocks func()
		expectedCode int
		expectedBody string
	}{
		{
			name:    "success",
			payload: `{"dish_id":1,"order_id":99,"restaurant_id":10,"rating":5,"comment":"Great!"}`,
			prepareMocks: func() {
				mockSvc.On("CreateOrUpdate", mock.Anything, mock.Anything).
					Return(nil).Once()
			},
			expectedCode: http.StatusOK,
			expectedBody: `"rating":5`,
		},
		{
			name:         "invalid_json",
			payload:      `bad json`,
			prepareMocks: func() {},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:    "dish_not_in_order",
			payload: `{"dish_id":1,"order_id":99,"restaurant_id":10,"rating":3}`,
			prepareMocks: func() {
				mockSvc.On("CreateOrUpdate", mock.Anything, mock.Anything).
					Return(service.ErrDishNotInOrder).Once()
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.prepareMocks()
			req := httptest.NewRequest("POST", "/api/restaurants/10/dishes/1/reviews", bytes.NewBufferString(testCase.payload))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
			if testCase.expectedBody != "" {
				assert.Contains(t, recorder.Body.String(), testCase.expectedBody)
			}
		})
	}
}

func TestHandler_getDishReviews(t *testing.T) {
	mockSvc := mocks.NewReviewServiceInterface(t)
	router := setupTestRouter(mockSvc)

	expectedReviews := []domain.Review{
		{DishID: 1, OrderID: 99, Rating: 5},
		{DishID: 1, OrderID: 100, Rating: 4},
	}

	mockSvc.On("ListDishReviews", 1, 10).Return(expectedReviews, nil).Once()

	req := httptest.NewRequest("GET", "/api/restaurants/10/dishes/1/reviews", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	var reviews []domain.Review
	json.NewDecoder(recorder.Body).Decode(&reviews)
	assert.Len(t, reviews, 2)
}

func TestHandler_createBulkReviews(t *testing.T) {
	mockSvc := mocks.NewReviewServiceInterface(t)
	router := setupTestRouter(mockSvc)

	tests := []struct {
		name         string
		payload      string
		prepareMocks func()
		expectedCode int
	}{
		{
			name:    "success_partial",
			payload: `{"check_id":99,"restaurant_id":10,"reviews":[{"dish_id":1,"rating":5},{"dish_id":2,"rating":3}]}`,
			prepareMocks: func() {
				mockSvc.On("CreateOrUpdate", mock.Anything, mock.Anything).
					Return(nil).Once()
				mockSvc.On("CreateOrUpdate", mock.Anything, mock.Anything).
					Return(errors.New("validation failed")).Once()
			},
			expectedCode: http.StatusCreated,
		},
		{
			name:         "missing_fields",
			payload:      `{"check_id":0,"restaurant_id":0,"reviews":[]}`,
			prepareMocks: func() {},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.prepareMocks()
			req := httptest.NewRequest("POST", "/api/reviews", bytes.NewBufferString(testCase.payload))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, testCase.expectedCode, recorder.Code)
		})
	}
}
