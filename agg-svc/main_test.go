package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// TestUpdateDishRating_Success ensures that when SQL queries succeed,
// Redis is updated without error.
func TestUpdateDishRating_Success(t *testing.T) {
	var (
		mockDB  *sql.DB
		sqlMock sqlmock.Sqlmock
		err     error
	)

	mockDB, sqlMock, err = sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	dishID := 1
	restaurantID := 2

	// Expect UPDATE aggregates
	sqlMock.ExpectExec("UPDATE dishes").
		WithArgs(dishID, restaurantID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect SELECT avg_rating, review_count
	rows := sqlmock.NewRows([]string{"avg_rating", "review_count"}).
		AddRow(4.5, 10)
	sqlMock.ExpectQuery("SELECT COALESCE\\(avg_rating, 0\\), COALESCE\\(review_count, 0\\)").
		WithArgs(dishID, restaurantID).
		WillReturnRows(rows)

	// Minimal redis client using redis.NewClient with a fake process function.
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})
	ctx = context.Background()

	if err := updateDishRating(dishID, restaurantID, 5); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// TestUpdateDishRating_UpdateError covers the branch when UPDATE fails.
func TestUpdateDishRating_UpdateError(t *testing.T) {
	mockDB, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	sqlMock.ExpectExec("UPDATE dishes").
		WithArgs(1, 1).
		WillReturnError(errors.New("update failed"))

	err = updateDishRating(1, 1, 5)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

// TestUpdateAnalytics_Success verifies normal Redis and DB interaction.
func TestUpdateAnalytics_Success(t *testing.T) {
	mockDB, sqlMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db = mockDB

	rows := sqlmock.NewRows([]string{"avg_rating"}).
		AddRow(4.2)
	sqlMock.ExpectQuery("SELECT COALESCE\\(avg_rating, 0\\)").
		WithArgs(1, 2).
		WillReturnRows(rows)

	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:0",
	})
	ctx = context.Background()

	if err := updateAnalytics(1, 2); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := sqlMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// TestProcessReviewMessage_Success ensures complete flow works
func TestProcessReviewMessage_Success(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	mr := miniredis.RunT(t)
	rdb = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer mr.Close()

	// Expect UPDATE and SELECT queries
	mock.ExpectExec("UPDATE dishes").
		WithArgs(1, 10).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rows := sqlmock.NewRows([]string{"avg_rating", "review_count"}).
		AddRow(4.5, 10)
	mock.ExpectQuery("SELECT COALESCE\\(avg_rating, 0\\), COALESCE\\(review_count, 0\\)").
		WithArgs(1, 10).
		WillReturnRows(rows)
	mock.ExpectQuery("SELECT COALESCE\\(avg_rating, 0\\)").
		WithArgs(1, 10).
		WillReturnRows(sqlmock.NewRows([]string{"avg_rating"}).AddRow(4.5))

	msg := KafkaMessage{
		Type:         "new_review",
		DishID:       1,
		RestaurantID: 10,
		Rating:       5,
	}
	processReviewMessage(msg)

	stats, _ := rdb.HGetAll(context.Background(), "dish:10:1").Result()
	if stats["avg_rating"] != "4.5" {
		t.Errorf("Expected avg_rating=4.5, got %s", stats["avg_rating"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// TestProcessReviewMessage_UpdateError covers update failure path
func TestProcessReviewMessage_UpdateError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	mock.ExpectExec("UPDATE dishes").
		WithArgs(1, 10).
		WillReturnError(errors.New("db error"))

	msg := KafkaMessage{
		Type:         "new_review",
		DishID:       1,
		RestaurantID: 10,
		Rating:       5,
	}

	// Should not panic
	processReviewMessage(msg)

	// Error should be logged but not crash
}

// TestProcessReviewMessage_InvalidType ignores unknown message types
func TestProcessReviewMessage_InvalidType(t *testing.T) {
	msg := KafkaMessage{
		Type:         "unknown_type",
		DishID:       1,
		RestaurantID: 10,
		Rating:       5,
	}

	// Should not panic for unknown types
	processReviewMessage(msg)
}

// TestUpdateDishRating_DBError handles database connection failures
func TestUpdateDishRating_DBError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	mock.ExpectExec("UPDATE dishes").
		WithArgs(1, 10).
		WillReturnError(errors.New("connection lost"))

	err = updateDishRating(1, 10, 5)
	if err == nil {
		t.Error("Expected error for DB failure, got nil")
	}
}

// TestUpdateAnalytics_RedisError ensures Redis errors don't crash service
func TestUpdateAnalytics_RedisError(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()
	db = mockDB

	// Use invalid Redis address
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:9999"})

	rows := sqlmock.NewRows([]string{"avg_rating"}).AddRow(4.2)
	mock.ExpectQuery("SELECT COALESCE\\(avg_rating, 0\\)").
		WithArgs(1, 10).
		WillReturnRows(rows)

	err = updateAnalytics(1, 10)
	if err != nil {
		t.Logf("Expected Redis error (non-critical): %v", err)
	}
}
