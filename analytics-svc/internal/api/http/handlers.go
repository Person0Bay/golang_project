package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"overcooked-simplified/analytics-svc/internal/service"

	"github.com/gorilla/mux"
)

type Handler struct {
	Analytics service.AnalyticsInterface
}

func NewHandler(svc service.AnalyticsInterface) *Handler {
	return &Handler{Analytics: svc}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")
	r.HandleFunc("/api/analytics/top-today", h.getTopToday).Methods("GET")
	r.HandleFunc("/api/analytics/top-alltime", h.getTopAllTime).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/analytics", h.getAnalytics).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/stats", h.getDishStats).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/top-dishes", h.getTopDishes).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/analytics/rating-distribution", h.getRatingDistribution).Methods("GET")
	r.HandleFunc("/api/analytics/rating-distribution", h.getGlobalRatingDistribution).Methods("GET")
}

func (h *Handler) getTopToday(w http.ResponseWriter, r *http.Request) {
	data, err := h.Analytics.TopToday()
	if err != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) getTopAllTime(w http.ResponseWriter, r *http.Request) {
	data, err := h.Analytics.TopAllTime()
	if err != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) getAnalytics(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}
	response := h.Analytics.AnalyticsForRestaurant(restaurantID, period)
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) getDishStats(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])
	stats, err := h.Analytics.DishStats(restaurantID, dishID)
	if err != nil || stats == nil {
		http.Error(w, "Dish stats not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) getTopDishes(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limitStr = "10"
	}
	limit, _ := strconv.Atoi(limitStr)
	data, _ := h.Analytics.TopDishes(restaurantID, limit)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) getRatingDistribution(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	data, _ := h.Analytics.RatingDistribution(restaurantID)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) getGlobalRatingDistribution(w http.ResponseWriter, r *http.Request) {
	data, _ := h.Analytics.GlobalDistribution()
	json.NewEncoder(w).Encode(data)
}
