package domain

import "time"

type KafkaMessage struct {
	Type         string    `json:"type"`
	DishID       int       `json:"dish_id"`
	RestaurantID int       `json:"restaurant_id"`
	OrderID      int       `json:"order_id"`
	Rating       int       `json:"rating"`
	Timestamp    time.Time `json:"timestamp"`
}
