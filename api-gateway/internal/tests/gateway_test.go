package tests

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overcooked-simplified/api-gateway/internal/gateway"
	"overcooked-simplified/api-gateway/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGateway_HealthCheck(t *testing.T) {
	gw := gateway.NewGateway(gateway.Config{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	gw.HealthCheck(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]string
	json.NewDecoder(rr.Body).Decode(&body)
	assert.Equal(t, "healthy", body["status"])
	assert.Equal(t, "api-gateway", body["service"])
}

func TestGateway_RouteHandler_CafeMenu(t *testing.T) {
	mockClient := mocks.NewHTTPClient(t)
	gw := gateway.NewGateway(gateway.Config{
		DishSvcURL: "http://dish-svc",
	}, mockClient)

	mockResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`[{"id":1,"name":"Pizza"}]`)),
		Header:     make(http.Header),
	}
	mockResp.Header.Set("Content-Type", "application/json")

	mockClient.On("Do", mock.Anything).Return(mockResp, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/cafe/10/menu", nil)
	rr := httptest.NewRecorder()

	gw.RouteHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Pizza")
	mockClient.AssertExpectations(t)
}

func TestGateway_RouteHandler_UnknownAPI(t *testing.T) {
	gw := gateway.NewGateway(gateway.Config{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rr := httptest.NewRecorder()

	gw.RouteHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestGateway_RouteHandler_ProxyError(t *testing.T) {
	mockClient := mocks.NewHTTPClient(t)
	gw := gateway.NewGateway(gateway.Config{
		DishSvcURL: "http://invalid",
	}, mockClient)

	mockClient.On("Do", mock.Anything).Return(nil, errors.New("connection failed")).Once()

	req := httptest.NewRequest(http.MethodGet, "/api/restaurants", nil)
	rr := httptest.NewRecorder()

	gw.RouteHandler(rr, req)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
}

func TestGateway_RouteHandler_ReviewsRoute(t *testing.T) {
	mockClient := mocks.NewHTTPClient(t)
	gw := gateway.NewGateway(gateway.Config{
		RateSvcURL: "http://rate-svc",
	}, mockClient)

	mockResp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(`{"status":"created"}`)),
		Header:     make(http.Header),
	}

	mockClient.On("Do", mock.Anything).Return(mockResp, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/api/reviews", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	gw.RouteHandler(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}
