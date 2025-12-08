package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthCheck verifies the basic JSON payload and status.
func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	healthCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "healthy" || body["service"] != "api-gateway" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

// TestRouteHandler_CafeMenu checks that cafe menu routes are proxied.
func TestRouteHandler_CafeMenu(t *testing.T) {
	backendPath := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	dishSvcURL = ts.URL

	req := httptest.NewRequest(http.MethodGet, "/api/cafe/10/menu", nil)
	rr := httptest.NewRecorder()

	routeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if backendPath != "/api/restaurants/10/dishes" {
		t.Fatalf("expected rewritten path /api/restaurants/10/dishes, got %s", backendPath)
	}
}

// TestRouteHandler_UnknownAPI ensures unknown /api routes return 404.
func TestRouteHandler_UnknownAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rr := httptest.NewRecorder()

	routeHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// TestRouteHandler_ReviewsRoute proxies to rate-svc
func TestRouteHandler_ReviewsRoute(t *testing.T) {
	received := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	rateSvcURL = ts.URL

	req := httptest.NewRequest(http.MethodPost, "/api/reviews", nil)
	rr := httptest.NewRecorder()

	routeHandler(rr, req)

	if !received {
		t.Fatal("Expected request to be proxied to rate-svc")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// TestRouteHandler_ProxyError handles backend service errors
func TestRouteHandler_ProxyError(t *testing.T) {
	dishSvcURL = "http://localhost:99999" // Invalid URL

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants", nil)
	rr := httptest.NewRecorder()

	routeHandler(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rr.Code)
	}
}
