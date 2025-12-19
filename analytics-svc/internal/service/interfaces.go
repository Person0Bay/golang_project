package service

import (
	"overcooked-simplified/analytics-svc/internal/domain"
)

type AnalyticsInterface interface {
	TopToday() ([]domain.DishAnalytics, error)
	TopAllTime() ([]domain.DishAnalytics, error)
	AnalyticsForRestaurant(restaurantID int, period string) domain.AnalyticsResponse
	DishStats(restaurantID, dishID int) (map[string]interface{}, error)
	TopDishes(restaurantID, limit int) ([]domain.DishAnalytics, error)
	RatingDistribution(restaurantID int) (map[string]int, error)
	GlobalDistribution() (map[string]int, error)
}

var _ AnalyticsInterface = (*AnalyticsService)(nil)
