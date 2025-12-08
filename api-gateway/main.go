package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

var (
	dishSvcURL      = os.Getenv("DISH_SVC_URL")
	rateSvcURL      = os.Getenv("RATE_SVC_URL")
	analyticsSvcURL = os.Getenv("ANALYTICS_SVC_URL")
)

func proxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
	log.Printf("PROXY: %s %s -> %s%s", r.Method, r.URL.Path, targetURL, r.URL.Path)

	url := targetURL + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, url, r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to create request: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, v := range r.Header {
		req.Header[k] = v
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to proxy to %s: %v", targetURL, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("ERROR: Failed to copy response: %v", err)
	}
}

func routeHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("ROUTE: %s %s", r.Method, path)

	// Route rewrites that keep service boundaries transparent.

	// Cafe menu route: /api/cafe/{id}/menu -> /api/restaurants/{id}/dishes
	if strings.HasPrefix(path, "/api/cafe/") && strings.Contains(path, "/menu") {
		parts := strings.Split(path, "/")
		if len(parts) >= 5 && parts[2] == "cafe" {
			cafeId := parts[3]
			r.URL.Path = fmt.Sprintf("/api/restaurants/%s/dishes", cafeId)
			log.Printf("[GATEWAY] Rewrote cafe menu path to: %s", r.URL.Path)
			proxyRequest(w, r, dishSvcURL)
			return
		}
	}

	// Redirect /api/cafes -> /api/restaurants
	if (path == "/api/cafes" || path == "/api/cafes/") && r.Method == "GET" {
		r.URL.Path = "/api/restaurants"
		log.Printf("[GATEWAY] Rewriting %s to /api/restaurants", path)
		proxyRequest(w, r, dishSvcURL)
		return
	}

	// QR code and order routes
	if strings.HasPrefix(path, "/api/orders/") && strings.HasSuffix(path, "/qrcode") {
		proxyRequest(w, r, dishSvcURL)
		return
	}

	// Global analytics routes
	if strings.HasPrefix(path, "/api/analytics/") {
		proxyRequest(w, r, analyticsSvcURL)
		return
	}

	// Restaurant-specific analytics
	if strings.HasPrefix(path, "/api/restaurants/") && strings.Contains(path, "/analytics/") {
		proxyRequest(w, r, analyticsSvcURL)
		return
	}

	// Reviews routes
	if strings.Contains(path, "/reviews") {
		proxyRequest(w, r, rateSvcURL)
		return
	}

	// Order/check management
	if strings.HasPrefix(path, "/api/check") || strings.HasPrefix(path, "/api/orders") {
		proxyRequest(w, r, dishSvcURL)
		return
	}

	// Restaurant routes
	if path == "/api/restaurants" || strings.HasPrefix(path, "/api/restaurants/") {
		proxyRequest(w, r, dishSvcURL)
		return
	}

	// Bulk reviews endpoint
	if path == "/api/reviews" && r.Method == "POST" {
		proxyRequest(w, r, rateSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/") {
		log.Printf("[GATEWAY] Unmatched API route: %s", path)
		http.Error(w, "API route not found", http.StatusNotFound)
		return
	}

	// Serve frontend
	http.ServeFile(w, r, "./frontend/index.html")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "api-gateway",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Provide defaults for local runs when env vars are missing.
	if dishSvcURL == "" {
		dishSvcURL = "http://localhost:8081"
	}
	if rateSvcURL == "" {
		rateSvcURL = "http://localhost:8082"
	}
	if analyticsSvcURL == "" {
		analyticsSvcURL = "http://localhost:8083"
	}

	r := mux.NewRouter()
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.PathPrefix("/api/").HandlerFunc(routeHandler)

	// Serve static frontend assets.
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend/"))))
	r.PathPrefix("/").HandlerFunc(routeHandler)

	// Configure CORS to support local dev URLs.
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080", "http://127.0.0.1:8080", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	log.Println("API Gateway starting on port 8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
