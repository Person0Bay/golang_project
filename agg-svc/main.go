package main

import (
	"context"
	"overcooked-simplified/agg-svc/internal/service"
	"overcooked-simplified/agg-svc/internal/storage"

	"overcooked-simplified/config"
)

func main() {
	db := config.MustInitPostgres()
	defer db.Close()

	rdb := config.MustInitRedis()
	defer rdb.Close()

	store := storage.NewStore(db, rdb)
	reader := config.NewKafkaReader("reviews", "agg-svc-consumer")
	defer reader.Close()

	consumer := service.NewConsumer(reader, store)
	consumer.Start(context.Background())
}
