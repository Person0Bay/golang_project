package storage

import (
	"context"
	"encoding/json"
	"strconv"

	"overcooked-simplified/rate-svc/internal/domain"

	"github.com/segmentio/kafka-go"
)

type KafkaPublisher struct {
	Writer *kafka.Writer
}

func NewKafkaPublisher(writer *kafka.Writer) *KafkaPublisher {
	return &KafkaPublisher{Writer: writer}
}

func (p *KafkaPublisher) PublishReview(ctx context.Context, msg domain.KafkaMessage) error {
	payload, _ := json.Marshal(msg)
	return p.Writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(strconv.Itoa(msg.DishID)),
		Value: payload,
	})
}
