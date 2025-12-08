package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// setupTestDB replaces the global db with a sqlmock-backed instance.
func setupTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	db = mockDB
	return mock, func() { mockDB.Close() }
}

// setupTestRedis installs a minimal Redis client on the global rdb.
func setupTestRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})
	ctx = context.Background()
}

func TestExtractRestaurantIDFromKey(t *testing.T) {
	if got := extractRestaurantIDFromKey("analytics:daily:2024-01-01:5:10"); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
	if got := extractRestaurantIDFromKey("invalid:key"); got != 0 {
		t.Fatalf("expected 0 for invalid key, got %d", got)
	}
}

func TestExtractRestaurantIDFromAllTimeKey(t *testing.T) {
	if got := extractRestaurantIDFromAllTimeKey("analytics:alltime:7"); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
	if got := extractRestaurantIDFromAllTimeKey("bad"); got != 0 {
		t.Fatalf("expected 0 for invalid key, got %d", got)
	}
}

func TestGetMostPopularDish_NoData(t *testing.T) {
	setupTestRedis()
	db = &sql.DB{} // unused in this path when Redis has no data

	date := time.Now().Format("2006-01-02")
	got, err := getMostPopularDish(1, date)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil when no Redis data, got %#v", got)
	}
}

func TestGetAnalytics_PeriodAll(t *testing.T) {
	setupTestRedis()
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	// Best rated dish query inside getBestRatedDish
	rows := sqlmock.NewRows([]string{"name", "review_count"}).
		AddRow("Pizza", 3)

	// Redis ZRevRangeWithScores will return empty (default), so we rely on DB.
	// For simplicity we skip mocking Redis internals here.

	mock.ExpectQuery("SELECT name, COALESCE\\(review_count, 0\\)").
		WithArgs(10, 1).
		WillReturnRows(rows)

	// Pretend Redis has one entry with dishID=10 and score 4.5
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})
	ctx = context.Background()

	// we can't easily stub ZRevRangeWithScores here without a real Redis,
	// so call getAnalytics with an empty response from getBestRatedDish by
	// temporarily overriding it.
	origGetBestRatedDish := getBestRatedDish
	getBestRatedDish = func(restaurantID int) (*DishAnalytics, error) {
		return &DishAnalytics{
			DishID:       10,
			DishName:     "Pizza",
			RestaurantID: restaurantID,
			Score:        4.5,
			ReviewCount:  3,
		}, nil
	}
	defer func() { getBestRatedDish = origGetBestRatedDish }()

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants/1/analytics?period=all", nil)
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1"})
	rr := httptest.NewRecorder()

	getAnalytics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp AnalyticsResponse
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.BestRatedDish == nil || resp.BestRatedDish.DishID != 10 {
		t.Fatalf("expected best rated dish with id 10, got %#v", resp.BestRatedDish)
	}
}

// TestGetTopToday_DBError verifies fallback when Redis is empty and DB fails
func TestGetTopToday_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	// Redis empty
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:0"})

	// DB error
	mock.ExpectQuery("SELECT d.id").
		WillReturnError(errors.New("db connection failed"))

	req := httptest.NewRequest(http.MethodGet, "/api/analytics/top-today", nil)
	rr := httptest.NewRecorder()

	getTopToday(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var dishes []DishAnalytics
	json.NewDecoder(rr.Body).Decode(&dishes)
	if dishes == nil {
		t.Error("Expected empty slice, got nil")
	}
}

// TestGetDishStats_NotFound returns 404 when dish not in Redis
func TestGetDishStats_NotFound(t *testing.T) {
	setupTestRedis()
	req := httptest.NewRequest(http.MethodGet, "/api/restaurants/1/dishes/99/stats", nil)
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1", "dishId": "99"})
	rr := httptest.NewRecorder()

	getDishStats(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
}

// TestGetRatingDistribution_Empty returns all zeros when no reviews
func TestGetRatingDistribution_Empty(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	rows := sqlmock.NewRows([]string{"rating", "count"})
	mock.ExpectQuery("SELECT rating, COUNT").
		WithArgs(1).
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants/1/analytics/rating-distribution", nil)
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1"})
	rr := httptest.NewRecorder()

	getRatingDistribution(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	var dist map[string]int
	json.NewDecoder(rr.Body).Decode(&dist)
	if dist["1"] != 0 || dist["5"] != 0 {
		t.Errorf("Expected all zeros, got %v", dist)
	}
}
