package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
)

// helper to install sqlmock-backed DB.
func setupDishTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	t.Helper()
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	db = mockDB
	return mock, func() { mockDB.Close() }
}

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	healthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["service"] != "dish-svc" {
		t.Fatalf("unexpected service field: %v", body["service"])
	}
}

func TestEnsureSchemaExecutesStatements(t *testing.T) {
	mock, cleanup := setupDishTestDB(t)
	defer cleanup()

	mock.ExpectExec("ALTER TABLE IF EXISTS restaurants").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE IF EXISTS orders").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := ensureSchema(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCreateRestaurant_Success(t *testing.T) {
	mock, cleanup := setupDishTestDB(t)
	defer cleanup()

	mock.ExpectQuery("INSERT INTO restaurants").
		WithArgs("Cafe", "Addr", "Desc").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, sql.NullTime{}.Time))

	body, _ := json.Marshal(Restaurant{Name: "Cafe", Address: "Addr", Description: "Desc"})
	req := httptest.NewRequest(http.MethodPost, "/api/restaurants", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	createRestaurant(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestGetRestaurant_NotFound(t *testing.T) {
	mock, cleanup := setupDishTestDB(t)
	defer cleanup()

	mock.ExpectQuery("SELECT id, name").
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants/99", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "99"})
	rr := httptest.NewRecorder()

	getRestaurant(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestGenerateOrderQRCode(t *testing.T) {
	data, err := generateOrderQRCode(123)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty QR code bytes")
	}
}

// TestCreateDish_Success validates dish creation flow
func TestCreateDish_Success(t *testing.T) {
	mock, cleanup := setupDishTestDB(t)
	defer cleanup()

	mock.ExpectQuery("INSERT INTO dishes").
		WithArgs(1, "Pasta", "Delicious", 12.99, "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, sql.NullTime{}.Time))

	body, _ := json.Marshal(Dish{Name: "Pasta", Description: "Delicious", Price: 12.99})
	req := httptest.NewRequest(http.MethodPost, "/api/restaurants/1/dishes", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1"})
	rr := httptest.NewRecorder()

	createDish(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// TestUploadDishImage_InvalidFileType checks file type validation
func TestUploadDishImage_InvalidFileType(t *testing.T) {
	body := &bytes.Buffer{}
	req := httptest.NewRequest(http.MethodPost, "/api/restaurants/1/dishes/1/image", body)
	req = mux.SetURLVars(req, map[string]string{"restaurantId": "1", "dishId": "1"})
	req.Header.Set("Content-Type", "multipart/form-data")
	rr := httptest.NewRecorder()

	uploadDishImage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestCreateOrder_ValidationError checks invalid order payload
func TestCreateOrder_ValidationError(t *testing.T) {
	body := []byte(`{"restaurant_id": 0, "items": []}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	createOrder(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
