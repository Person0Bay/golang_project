package domain

import "time"

type Restaurant struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type Dish struct {
	ID           int       `json:"dish_id"`
	RestaurantID int       `json:"restaurant_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Price        float64   `json:"price"`
	ImageURL     string    `json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
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
