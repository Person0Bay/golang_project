package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Config struct {
	DishSvcURL      string
	RateSvcURL      string
	AnalyticsSvcURL string
}

type Gateway struct {
	config Config
	client HTTPClient
}

func NewGateway(config Config, client HTTPClient) *Gateway {
	return &Gateway{
		config: config,
		client: client,
	}
}

func (g *Gateway) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "api-gateway",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (g *Gateway) ProxyRequest(w http.ResponseWriter, r *http.Request, targetURL string) {
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

	resp, err := g.client.Do(req)
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

func (g *Gateway) RouteHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("ROUTE: %s %s", r.Method, path)

	if strings.HasPrefix(path, "/api/cafe/") && strings.Contains(path, "/menu") {
		parts := strings.Split(path, "/")
		if len(parts) >= 5 && parts[2] == "cafe" {
			cafeID := parts[3]
			r.URL.Path = fmt.Sprintf("/api/restaurants/%s/dishes", cafeID)
			log.Printf("[GATEWAY] Rewrote cafe menu path to: %s", r.URL.Path)
			g.ProxyRequest(w, r, g.config.DishSvcURL)
			return
		}
	}

	if (path == "/api/cafes" || path == "/api/cafes/") && r.Method == "GET" {
		r.URL.Path = "/api/restaurants"
		log.Printf("[GATEWAY] Rewriting %s to /api/restaurants", path)
		g.ProxyRequest(w, r, g.config.DishSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/orders/") && strings.HasSuffix(path, "/qrcode") {
		g.ProxyRequest(w, r, g.config.DishSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/analytics/") {
		g.ProxyRequest(w, r, g.config.AnalyticsSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/restaurants/") && strings.Contains(path, "/analytics/") {
		g.ProxyRequest(w, r, g.config.AnalyticsSvcURL)
		return
	}

	if strings.Contains(path, "/reviews") {
		g.ProxyRequest(w, r, g.config.RateSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/check") || strings.HasPrefix(path, "/api/orders") {
		g.ProxyRequest(w, r, g.config.DishSvcURL)
		return
	}

	if path == "/api/restaurants" || strings.HasPrefix(path, "/api/restaurants/") {
		g.ProxyRequest(w, r, g.config.DishSvcURL)
		return
	}

	if path == "/api/reviews" && r.Method == "POST" {
		g.ProxyRequest(w, r, g.config.RateSvcURL)
		return
	}

	if strings.HasPrefix(path, "/api/") {
		log.Printf("[GATEWAY] Unmatched API route: %s", path)
		http.Error(w, "API route not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, "./frontend/index.html")
}

func (g *Gateway) SetupRoutes() http.Handler {
	r := mux.NewRouter()
	r.HandleFunc("/health", g.HealthCheck).Methods("GET")
	r.PathPrefix("/api/").HandlerFunc(g.RouteHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend/"))))
	r.PathPrefix("/").HandlerFunc(g.RouteHandler)
	return r
}
