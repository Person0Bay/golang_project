package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"overcooked-simplified/rate-svc/internal/domain"
	"overcooked-simplified/rate-svc/internal/service"

	"github.com/gorilla/mux"
)

type Handler struct {
	Reviews service.ReviewServiceInterface
}

func NewHandler(reviews service.ReviewServiceInterface) *Handler {
	return &Handler{Reviews: reviews}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/reviews", h.createReview).Methods("POST")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/reviews", h.getDishReviews).Methods("GET")
	r.HandleFunc("/api/reviews", h.createBulkReviews).Methods("POST")
}

func (h *Handler) createReview(w http.ResponseWriter, r *http.Request) {
	var review domain.Review
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Reviews.CreateOrUpdate(r.Context(), &review); err != nil {
		switch err {
		case service.ErrDishNotInOrder:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case service.ErrDuplicateReview:
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(review)
}

func (h *Handler) getDishReviews(w http.ResponseWriter, r *http.Request) {
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])

	reviews, err := h.Reviews.ListDishReviews(dishID, restaurantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}

func (h *Handler) createBulkReviews(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CheckID      int `json:"check_id"`
		RestaurantID int `json:"restaurant_id"`
		Reviews      []struct {
			DishID  int    `json:"dish_id"`
			Rating  int    `json:"rating"`
			Comment string `json:"comment"`
		} `json:"reviews"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	if payload.CheckID == 0 || payload.RestaurantID == 0 || len(payload.Reviews) == 0 {
		http.Error(w, "Missing check_id, restaurant_id or reviews", http.StatusBadRequest)
		return
	}

	type reviewResult struct {
		DishID  int    `json:"dish_id"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}

	results := make([]reviewResult, 0, len(payload.Reviews))
	successCount := 0

	for _, incoming := range payload.Reviews {
		review := domain.Review{
			DishID:       incoming.DishID,
			OrderID:      payload.CheckID,
			RestaurantID: payload.RestaurantID,
			Rating:       incoming.Rating,
			Comment:      incoming.Comment,
		}

		err := h.Reviews.CreateOrUpdate(r.Context(), &review)
		if err != nil {
			results = append(results, reviewResult{
				DishID:  incoming.DishID,
				Status:  "error",
				Message: err.Error(),
			})
			continue
		}

		successCount++
		results = append(results, reviewResult{
			DishID: incoming.DishID,
			Status: "ok",
		})
	}

	if successCount == 0 {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"processed": results,
		"created":   successCount,
		"failed":    len(results) - successCount,
	})
}
