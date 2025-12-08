package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// helper to install sqlmock-backed DB.
func setupRateTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	db = mockDB
	return mock, func() { mockDB.Close() }
}

func setupRateTestRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})
}

func TestValidateDishInOrder_True(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()

	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(1, 2, 3).
		WillReturnRows(rows)

	ok, err := validateDishInOrder(1, 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}
}

func TestPersistReview_DishNotInOrder(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()
	setupRateTestRedis()

	// validateDishInOrder query returns false.
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(1, 2, 3).
		WillReturnRows(rows)

	review := &Review{
		DishID:       1,
		OrderID:      2,
		RestaurantID: 3,
		Rating:       5,
		Comment:      "Nice",
	}

	err := persistReview(review)
	if !errors.Is(err, errDishNotInOrder) {
		t.Fatalf("expected errDishNotInOrder, got %v", err)
	}
}

func TestCreateReview_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/restaurants/1/dishes/2/reviews", bytes.NewBufferString("bad json"))
	rr := httptest.NewRecorder()

	createReview(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetDishReviews_DBError(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, dish_id").
		WithArgs(2, 1).
		WillReturnError(sql.ErrConnDone)

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants/1/dishes/2/reviews", nil)
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1", "dishId": "2"})
	rr := httptest.NewRecorder()

	getDishReviews(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestCreateBulkReviews_MissingFields(t *testing.T) {
	body := `{"check_id":0,"restaurant_id":0,"reviews":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	createBulkReviews(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestPersistReview_KafkaError handles Kafka publish failure
func TestPersistReview_KafkaError(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()
	setupRateTestRedis()

	// Simulate Kafka not available
	kafkaWriter = &kafka.Writer{
		Addr:  kafka.TCP("localhost:99999"),
		Topic: "reviews",
	}

	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(1, 2, 3).
		WillReturnRows(rows)

	mock.ExpectQuery("SELECT id FROM reviews").
		WithArgs(1, 2, 3).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery("INSERT INTO reviews").
		WithArgs(1, 2, 3, 5, "Test").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, time.Now()))

	review := &Review{
		DishID:       1,
		OrderID:      2,
		RestaurantID: 3,
		Rating:       5,
		Comment:      "Test",
	}

	err := persistReview(review)
	if err == nil {
		t.Error("Expected Kafka error, got nil")
	}
}

// TestCreateBulkReviews_PartialSuccess handles mixed success/failure
func TestCreateBulkReviews_PartialSuccess(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()

	// Setup working Redis with miniredis
	mr := miniredis.RunT(t)
	rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	// Disable Kafka to avoid network errors
	kafkaWriter = nil

	// First review: dish in order
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(1, 99, 10).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	// No existing review
	mock.ExpectQuery("SELECT id FROM reviews").
		WithArgs(1, 99, 10).
		WillReturnError(sql.ErrNoRows)

	// Insert review
	mock.ExpectQuery("INSERT INTO reviews").
		WithArgs(1, 99, 10, 5, "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, time.Now()))

	// Second review: dish not in order
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(2, 99, 10).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	body := `{"check_id":99,"restaurant_id":10,"reviews":[{"dish_id":1,"rating":5},{"dish_id":2,"rating":3}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/reviews", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	createBulkReviews(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["created"].(float64) != 1 || resp["failed"].(float64) != 1 {
		t.Errorf("Expected 1 created, 1 failed, got %v", resp)
	}
}

// TestValidateDishInOrder_DBError handles database validation error
func TestValidateDishInOrder_DBError(t *testing.T) {
	mock, cleanup := setupRateTestDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(1, 2, 3).
		WillReturnError(errors.New("db error"))

	_, err := validateDishInOrder(1, 2, 3)
	if err == nil {
		t.Error("Expected DB error")
	}
}
