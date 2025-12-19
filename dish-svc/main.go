package main

import (
	"log"
	httpapi "overcooked-simplified/dish-svc/internal/api/http"
	"overcooked-simplified/dish-svc/internal/service"
	"overcooked-simplified/dish-svc/internal/storage"

	"overcooked-simplified/config"
)

func main() {
	db := config.MustInitPostgres()
	defer db.Close()

	repo := storage.NewPostgresRepository(db)
	if err := repo.EnsureSchema(); err != nil {
		log.Fatal("Failed to ensure schema:", err)
	}

	restSvc := service.NewRestaurantService(repo)
	dishSvc := service.NewDishService(repo)
	qrGen := service.DefaultQRGenerator{BaseURL: "http://localhost"}
	orderSvc := service.NewOrderService(repo, qrGen)

	handler := httpapi.NewHandler(restSvc, dishSvc, orderSvc)
	router := httpapi.NewRouter(handler)

	httpapi.StartServer(":8081", router)
}
