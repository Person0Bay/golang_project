package main

import (
	"log"
	"net/http"
	"os"

	"overcooked-simplified/api-gateway/internal/gateway"

	"github.com/rs/cors"
)

func main() {
	config := gateway.Config{
		DishSvcURL:      getEnv("DISH_SVC_URL", "http://localhost:8081"),
		RateSvcURL:      getEnv("RATE_SVC_URL", "http://localhost:8082"),
		AnalyticsSvcURL: getEnv("ANALYTICS_SVC_URL", "http://localhost:8083"),
	}

	gw := gateway.NewGateway(config, &http.Client{})

	r := gw.SetupRoutes()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080", "http://127.0.0.1:8080", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	handler := c.Handler(r)

	log.Println("API Gateway starting on port 8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
