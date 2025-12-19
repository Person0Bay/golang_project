package main

import (
	httpapi "overcooked-simplified/analytics-svc/internal/api/http"
	"overcooked-simplified/analytics-svc/internal/service"

	"overcooked-simplified/config"
)

func main() {
	db := config.MustInitPostgres()
	defer db.Close()

	rdb := config.MustInitRedis()
	defer rdb.Close()

	analyticsSvc := service.NewAnalyticsService(db, rdb)
	handler := httpapi.NewHandler(analyticsSvc)
	router := httpapi.NewRouter(handler)

	httpapi.StartServer(":8083", router)
}
