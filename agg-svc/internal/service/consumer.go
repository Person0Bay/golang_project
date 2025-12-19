package service

import (
	"context"
	"encoding/json"
	"log"

	"overcooked-simplified/agg-svc/internal/domain"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	Reader *kafka.Reader
	Store  StoreInterface
}

func NewConsumer(reader *kafka.Reader, store StoreInterface) *Consumer {
	return &Consumer{
		Reader: reader,
		Store:  store,
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Println("Starting Aggregation Service consumer...")
	for {
		message, err := c.Reader.ReadMessage(ctx)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			continue
		}

		var msg domain.KafkaMessage
		if err := json.Unmarshal(message.Value, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		if msg.Type == "new_review" {
			c.ProcessReview(msg)
		}
	}
}

func (c *Consumer) ProcessReview(msg domain.KafkaMessage) {
	if msg.Type != "new_review" {
		return
	}
	log.Printf("Processing review: DishID=%d, RestaurantID=%d, Rating=%d",
		msg.DishID, msg.RestaurantID, msg.Rating)

	if err := c.Store.UpdateDishRating(msg.DishID, msg.RestaurantID); err != nil {
		log.Printf("Error updating dish rating: %v", err)
		return
	}

	if err := c.Store.UpdateAnalytics(msg.DishID, msg.RestaurantID); err != nil {
		log.Printf("Error updating analytics: %v", err)
		return
	}

	log.Printf("Successfully processed review for dish %d", msg.DishID)
}
