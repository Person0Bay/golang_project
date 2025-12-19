package main

import (
	httpapi "overcooked-simplified/rate-svc/internal/api/http"
	"overcooked-simplified/rate-svc/internal/service"
	"overcooked-simplified/rate-svc/internal/storage"
	"time"

	"overcooked-simplified/config"
)

func main() {
	db := config.MustInitPostgres()
	defer db.Close()

	rdb := config.MustInitRedis()
	defer rdb.Close()

	kafkaWriter := config.NewKafkaWriter("reviews")
	defer kafkaWriter.Close()

	repository := storage.NewPostgresRepository(db)
	cache := storage.NewRedisCache(rdb, 24*7*time.Hour)
	publisher := storage.NewKafkaPublisher(kafkaWriter)
	reviewService := service.NewReviewService(repository, cache, publisher)

	handler := httpapi.NewHandler(reviewService)
	router := httpapi.NewRouter(handler)

	httpapi.StartServer(":8082", router)
}
