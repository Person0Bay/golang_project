package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/skip2/go-qrcode"
)

type Dish struct {
	ID           int       `json:"id"`
	RestaurantID int       `json:"restaurant_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Price        float64   `json:"price"`
	ImageURL     string    `json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
}

type Restaurant struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type Order struct {
	ID             int         `json:"id"`
	RestaurantID   int         `json:"restaurant_id"`
	RestaurantName string      `json:"cafe_name"`
	TotalAmount    float64     `json:"total_amount"`
	Status         string      `json:"status"`
	QRCode         string      `json:"qr_code,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	Items          []OrderItem `json:"items"`
}

type OrderItem struct {
	DishID   int     `json:"dish_id"`
	DishName string  `json:"dish_name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

var db *sql.DB

func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "dish-svc",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	if err = ensureSchema(); err != nil {
		log.Fatal("Failed to ensure schema:", err)
	}
}

func ensureSchema() error {
	statements := []string{
		"ALTER TABLE IF EXISTS restaurants ADD COLUMN IF NOT EXISTS description TEXT",
		"ALTER TABLE IF EXISTS orders ADD COLUMN IF NOT EXISTS qr_code BYTEA",
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("ensure schema `%s`: %w", stmt, err)
		}
	}

	return nil
}

func createRestaurant(w http.ResponseWriter, r *http.Request) {
	var restaurant Restaurant
	var err error

	err = json.NewDecoder(r.Body).Decode(&restaurant)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = db.QueryRow("INSERT INTO restaurants (name, address, description) VALUES ($1, $2, $3) RETURNING id, created_at",
		restaurant.Name, restaurant.Address, restaurant.Description).Scan(&restaurant.ID, &restaurant.CreatedAt)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(restaurant)
}

func getRestaurants(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
        SELECT id, name, COALESCE(address, ''), COALESCE(description, ''), COALESCE(image_url, ''), created_at
        FROM restaurants
        ORDER BY created_at DESC`)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	restaurants := []Restaurant{}
	for rows.Next() {
		var r Restaurant
		err := rows.Scan(&r.ID, &r.Name, &r.Address, &r.Description, &r.ImageURL, &r.CreatedAt)
		if err != nil {
			continue
		}
		restaurants = append(restaurants, r)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(restaurants)
}

func createDish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	var dish Dish
	var err error

	err = json.NewDecoder(r.Body).Decode(&dish)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dish.RestaurantID = restaurantID

	err = db.QueryRow("INSERT INTO dishes (restaurant_id, name, description, price, image_url) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at",
		dish.RestaurantID, dish.Name, dish.Description, dish.Price, dish.ImageURL).Scan(&dish.ID, &dish.CreatedAt)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func getDish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])
	dishID, _ := strconv.Atoi(vars["dishId"])

	var dish Dish
	var err error

	err = db.QueryRow("SELECT id, restaurant_id, name, description, price, COALESCE(image_url, ''), created_at FROM dishes WHERE id = $1 AND restaurant_id = $2", dishID, restaurantID).
		Scan(&dish.ID, &dish.RestaurantID, &dish.Name, &dish.Description, &dish.Price, &dish.ImageURL, &dish.CreatedAt)

	if err != nil {
		http.Error(w, "Dish not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func getRestaurantDishes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])

	log.Printf("[dish-svc] fetching dishes for restaurant_id=%d", restaurantID)

	rows, err := db.Query(`
		SELECT id, restaurant_id, name, description, price, COALESCE(image_url, ''), created_at
		FROM dishes
		WHERE restaurant_id = $1
		ORDER BY created_at DESC`, restaurantID)

	if err != nil {
		log.Printf("[dish-svc] database error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	dishes := []Dish{}
	for rows.Next() {
		var dish Dish
		var err error

		err = rows.Scan(&dish.ID, &dish.RestaurantID, &dish.Name, &dish.Description, &dish.Price, &dish.ImageURL, &dish.CreatedAt)
		if err != nil {
			log.Printf("[dish-svc] row scan error: %v", err)
			continue
		}

		dishes = append(dishes, dish)
	}

	log.Printf("[dish-svc] found %d dishes for restaurant %d", len(dishes), restaurantID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dishes)
}

func uploadDishImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])
	dishID, _ := strconv.Atoi(vars["dishId"])

	var err error

	err = r.ParseMultipartForm(10 << 20)
	if err != nil {
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
	err = os.MkdirAll(uploadDir, 0755)
	if err != nil {
		http.Error(w, "Failed to create upload directory", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("dish_%d_%d_%s", restaurantID, dishID, handler.Filename)
	filepath := fmt.Sprintf("%s/%s", uploadDir, filename)

	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	imageURL := fmt.Sprintf("/uploads/%s", filename)
	_, err = db.Exec("UPDATE dishes SET image_url = $1 WHERE id = $2 AND restaurant_id = $3",
		imageURL, dishID, restaurantID)

	if err != nil {
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

func createOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	var err error

	err = json.NewDecoder(r.Body).Decode(&order)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Basic payload validation
	if order.RestaurantID <= 0 || len(order.Items) == 0 {
		http.Error(w, "Invalid order payload", http.StatusBadRequest)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Database transaction error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
		INSERT INTO orders (restaurant_id, total_amount, status, qr_code)
		VALUES ($1, $2, 'completed', NULL)
		RETURNING id, created_at
	`, order.RestaurantID, order.TotalAmount).Scan(&order.ID, &order.CreatedAt)

	if err != nil {
		http.Error(w, "Failed to create order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	qrCode, err := generateOrderQRCode(order.ID)
	if err != nil {
		log.Printf("WARNING: Failed to generate QR code: %v", err)
	} else {
		if _, err := tx.Exec(`UPDATE orders SET qr_code = $1 WHERE id = $2`, qrCode, order.ID); err != nil {
			log.Printf("WARNING: Failed to store QR code for order %d: %v", order.ID, err)
		}
	}

	order.QRCode = fmt.Sprintf("/api/orders/%d/qrcode", order.ID)
	if order.TotalAmount == 0 {
		order.TotalAmount = order.TotalAmount // Keep field present in JSON
	}

	for _, item := range order.Items {
		_, execErr := tx.Exec(`
			INSERT INTO order_items (order_id, dish_id, quantity, price)
			VALUES ($1, $2, $3, $4)
		`, order.ID, item.DishID, item.Quantity, item.Price)
		if execErr != nil {
			log.Printf("WARNING: Failed to insert order item: %v", execErr)
			continue
		}
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, "Failed to commit transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func getOrderQRCode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID, _ := strconv.Atoi(vars["id"])

	var qrCode []byte
	var err error

	err = db.QueryRow("SELECT qr_code FROM orders WHERE id = $1", orderID).Scan(&qrCode)
	if err == sql.ErrNoRows {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(qrCode) == 0 {
		qrCode, err = generateOrderQRCode(orderID)
		if err != nil {
			http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
			return
		}
		_, err = db.Exec("UPDATE orders SET qr_code = $1 WHERE id = $2", qrCode, orderID)
		if err != nil {
			log.Printf("WARNING: Failed to cache regenerated QR code: %v", err)
		}
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	w.Write(qrCode)
}

func generateOrderQRCode(orderID int) ([]byte, error) {
	qrData := fmt.Sprintf("http://localhost/review.html?check_id=%d", orderID)
	return qrcode.Encode(qrData, qrcode.Medium, 256)
}

// GET /api/check/{id} - Return a specific order
func getOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID, _ := strconv.Atoi(vars["id"])

	var order Order
	var err error

	err = db.QueryRow(`
		SELECT id, restaurant_id, total_amount, status, created_at
		FROM orders WHERE id = $1
	`, orderID).Scan(&order.ID, &order.RestaurantID, &order.TotalAmount, &order.Status, &order.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var restaurantName string
	db.QueryRow("SELECT name FROM restaurants WHERE id = $1", order.RestaurantID).Scan(&restaurantName)
	order.RestaurantName = restaurantName

	rows, err := db.Query(`
		SELECT oi.dish_id, d.name, oi.quantity, oi.price
		FROM order_items oi
		JOIN dishes d ON oi.dish_id = d.id
		WHERE oi.order_id = $1
	`, orderID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	order.Items = []OrderItem{}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.DishID, &item.DishName, &item.Quantity, &item.Price); err != nil {
			continue
		}
		order.Items = append(order.Items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func getOrders(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, restaurant_id, total_amount, status, created_at
		FROM orders
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var order Order
		var err error

		err = rows.Scan(&order.ID, &order.RestaurantID, &order.TotalAmount, &order.Status, &order.CreatedAt)
		if err != nil {
			continue
		}

		var restaurantName string
		db.QueryRow("SELECT name FROM restaurants WHERE id = $1", order.RestaurantID).Scan(&restaurantName)
		order.RestaurantName = restaurantName

		orders = append(orders, order)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func updateRestaurant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var restaurant Restaurant
	json.NewDecoder(r.Body).Decode(&restaurant)

	err := db.QueryRow(
		"UPDATE restaurants SET name=$1, address=$2, description=$3 WHERE id=$4 RETURNING id, name, address, description, COALESCE(image_url, ''), created_at",
		restaurant.Name, restaurant.Address, restaurant.Description, id).
		Scan(&restaurant.ID, &restaurant.Name, &restaurant.Address, &restaurant.Description, &restaurant.ImageURL, &restaurant.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Restaurant not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(restaurant)
}

func deleteRestaurant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	result, err := db.Exec("DELETE FROM restaurants WHERE id=$1", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Restaurant not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func uploadRestaurantImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM restaurants WHERE id=$1)", id).Scan(&exists)
	if !exists {
		http.Error(w, "Restaurant not found", http.StatusNotFound)
		return
	}

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	os.MkdirAll("./uploads", 0755)
	filename := fmt.Sprintf("restaurant_%d_%s", id, handler.Filename)
	filepath := fmt.Sprintf("./uploads/%s", filename)

	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	imageURL := fmt.Sprintf("/uploads/%s", filename)
	db.Exec("UPDATE restaurants SET image_url=$1 WHERE id=$2", imageURL, id)

	json.NewEncoder(w).Encode(map[string]string{"image_url": imageURL})
}

func getRestaurant(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	var restaurant Restaurant
	err := db.QueryRow(`
		SELECT id, name, COALESCE(address, ''), COALESCE(description, ''), COALESCE(image_url, ''), created_at
		FROM restaurants
		WHERE id = $1`, id).
		Scan(&restaurant.ID, &restaurant.Name, &restaurant.Address, &restaurant.Description, &restaurant.ImageURL, &restaurant.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Restaurant not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(restaurant)
}

func updateDish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])
	dishID, _ := strconv.Atoi(vars["dishId"])

	var dish Dish
	err := json.NewDecoder(r.Body).Decode(&dish)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`
		UPDATE dishes
		SET name=$1, description=$2, price=$3
		WHERE id=$4 AND restaurant_id=$5`,
		dish.Name, dish.Description, dish.Price, dishID, restaurantID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dish.ID = dishID
	dish.RestaurantID = restaurantID

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dish)
}

func deleteDish(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	restaurantID, _ := strconv.Atoi(vars["restaurantId"])
	dishID, _ := strconv.Atoi(vars["dishId"])

	result, err := db.Exec("DELETE FROM dishes WHERE id=$1 AND restaurant_id=$2", dishID, restaurantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Dish not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
func main() {
	initDB()
	defer db.Close()

	r := mux.NewRouter()

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/api/restaurants", createRestaurant).Methods("POST")
	r.HandleFunc("/api/restaurants", getRestaurants).Methods("GET")
	r.HandleFunc("/api/restaurants/{id}", getRestaurant).Methods("GET")
	r.HandleFunc("/api/restaurants/{id}", updateRestaurant).Methods("PUT")
	r.HandleFunc("/api/restaurants/{id}", deleteRestaurant).Methods("DELETE")
	r.HandleFunc("/api/restaurants/{id}/image", uploadRestaurantImage).Methods("POST")

	r.HandleFunc("/api/restaurants/{restaurantId}/dishes", createDish).Methods("POST")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes", getRestaurantDishes).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", getDish).Methods("GET")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", updateDish).Methods("PUT")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}", deleteDish).Methods("DELETE")
	r.HandleFunc("/api/restaurants/{restaurantId}/dishes/{dishId}/image", uploadDishImage).Methods("POST")

	r.HandleFunc("/api/orders", createOrder).Methods("POST")
	r.HandleFunc("/api/orders", getOrders).Methods("GET")
	r.HandleFunc("/api/orders/{id}", getOrder).Methods("GET")
	r.HandleFunc("/api/orders/{id}/qrcode", getOrderQRCode).Methods("GET")
	r.HandleFunc("/api/check/{id}", getOrder).Methods("GET")

	handler := cors.Default().Handler(r)

	log.Println("Dish Service starting on port 8081")
	log.Fatal(http.ListenAndServe(":8081", handler))
}
