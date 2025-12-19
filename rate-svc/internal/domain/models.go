package domain

import "time"

type Review struct {
	ID           int       `json:"id"`
	DishID       int       `json:"dish_id"`
	OrderID      int       `json:"order_id"`
	RestaurantID int       `json:"restaurant_id"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type KafkaMessage struct {
	Type         string    `json:"type"`
	DishID       int       `json:"dish_id"`
	RestaurantID int       `json:"restaurant_id"`
	OrderID      int       `json:"order_id"`
	Rating       int       `json:"rating"`
	Timestamp    time.Time `json:"timestamp"`
}
