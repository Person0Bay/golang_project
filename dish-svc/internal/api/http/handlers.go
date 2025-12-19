package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"overcooked-simplified/dish-svc/internal/domain"
	"overcooked-simplified/dish-svc/internal/service"

	"github.com/gorilla/mux"
)

type Handler struct {
	Restaurants service.RestaurantServiceInterface
	Dishes      service.DishServiceInterface
	Orders      service.OrderServiceInterface
}

func NewHandler(restSvc service.RestaurantServiceInterface, dishSvc service.DishServiceInterface, orderSvc service.OrderServiceInterface) *Handler {
	return &Handler{
		Restaurants: restSvc,
		Dishes:      dishSvc,
		Orders:      orderSvc,
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/health", h.healthCheck).Methods("GET")

	r.HandleFunc("/api/restaurants", h.createRestaurant).Methods("POST")
	r.HandleFunc("/api/restaurants", h.getRestaurants).Methods("GET")
	r.HandleFunc("/api/restaurants/{id}", h.getRestaurant).Methods("GET")
	r.HandleFunc("/api/restaurants/{id}", h.updateRestaurant).Methods("PUT")
	r.HandleFunc("/api/restaurants/{id}", h.deleteRestaurant).Methods("DELETE")
	r.HandleFunc("/api/restaurants/{id}/image", h.uploadRestaurantImage).Methods("POST")

	r.HandleFunc("/api/restaurants/{restaurantId}/dishes", h.createDish).Methods("POST")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes", h.getRestaurantDishes).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", h.getDish).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", h.updateDish).Methods("PUT")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", h.deleteDish).Methods("DELETE")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/image", h.uploadDishImage).Methods("POST")

	r.HandleFunc("/api/orders", h.createOrder).Methods("POST")
	r.HandleFunc("/api/orders", h.getOrders).Methods("GET")
	r.HandleFunc("/api/orders/{id}", h.getOrder).Methods("GET")
	r.HandleFunc("/api/orders/{id}/qrcode", h.getOrderQRCode).Methods("GET")
	r.HandleFunc("/api/check/{id}", h.getOrder).Methods("GET")
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "dish-svc",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) createRestaurant(w http.ResponseWriter, r *http.Request) {
	var rest domain.Restaurant
	if err := json.NewDecoder(r.Body).Decode(&rest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.Restaurants.Create(&rest); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rest)
}

func (h *Handler) getRestaurants(w http.ResponseWriter, r *http.Request) {
	restaurants, err := h.Restaurants.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(restaurants)
}

func (h *Handler) getRestaurant(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	rest, err := h.Restaurants.Get(id)
	if err != nil {
		http.Error(w, "Restaurant not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rest)
}

func (h *Handler) updateRestaurant(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	var rest domain.Restaurant
	if err := json.NewDecoder(r.Body).Decode(&rest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rest.ID = id
	if err := h.Restaurants.Update(&rest); err != nil {
		if err.Error() == "sql: no rows in result set" {
			http.Error(w, "Restaurant not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rest)
}

func (h *Handler) deleteRestaurant(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	rows, err := h.Restaurants.Delete(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Restaurant not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) uploadRestaurantImage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	filename := "restaurant_" + strconv.Itoa(id) + "_" + handler.Filename
	path := filepath.Join(uploadDir, filename)

	dst, err := os.Create(path)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	imageURL := "/uploads/" + filename
	if err := h.Restaurants.UpdateImage(id, imageURL); err != nil {
		http.Error(w, "Failed to update restaurant", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"image_url": imageURL})
}

func (h *Handler) createDish(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	var dish domain.Dish
	if err := json.NewDecoder(r.Body).Decode(&dish); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dish.RestaurantID = restaurantID
	if err := h.Dishes.Create(&dish); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func (h *Handler) getRestaurantDishes(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishes, err := h.Dishes.List(restaurantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dishes)
}

func (h *Handler) getDish(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])
	dish, err := h.Dishes.Get(restaurantID, dishID)
	if err != nil {
		http.Error(w, "Dish not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func (h *Handler) updateDish(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])
	var dish domain.Dish
	if err := json.NewDecoder(r.Body).Decode(&dish); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dish.ID = dishID
	dish.RestaurantID = restaurantID
	if err := h.Dishes.Update(&dish); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func (h *Handler) deleteDish(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])
	rows, err := h.Dishes.Delete(restaurantID, dishID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Dish not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) uploadDishImage(w http.ResponseWriter, r *http.Request) {
	restaurantID, _ := strconv.Atoi(mux.Vars(r)["restaurantId"])
	dishID, _ := strconv.Atoi(mux.Vars(r)["dishId"])

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowedTypes[handler.Header.Get("Content-Type")] {
		http.Error(w, "Invalid file type. Only JPEG, PNG, GIF, WebP allowed", http.StatusBadRequest)
		return
	}

	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	filename := "dish_" + strconv.Itoa(restaurantID) + "_" + strconv.Itoa(dishID) + "_" + handler.Filename
	path := filepath.Join(uploadDir, filename)

	dst, err := os.Create(path)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	imageURL := "/uploads/" + filename
	if err := h.Dishes.UpdateImage(restaurantID, dishID, imageURL); err != nil {
		http.Error(w, "Failed to update dish", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"message":   "Image uploaded successfully",
		"image_url": imageURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	var order domain.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Orders.Create(&order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order.QRCode = h.Orders.QRLink(order.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.Atoi(mux.Vars(r)["id"])
	order, err := h.Orders.Get(orderID)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (h *Handler) getOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.Orders.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (h *Handler) getOrderQRCode(w http.ResponseWriter, r *http.Request) {
	orderID, _ := strconv.Atoi(mux.Vars(r)["id"])
	qrCode, err := h.Orders.GetQRCode(orderID)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if len(qrCode) == 0 {
		http.Error(w, "QR code not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(qrCode)
}
