package domain

type DishAnalytics struct {
	DishID       int     `json:"dish_id"`
	DishName     string  `json:"dish_name"`
	RestaurantID int     `json:"restaurant_id"`
	Score        float64 `json:"score"`
	ReviewCount  int     `json:"review_count"`
}

type AnalyticsResponse struct {
	MostPopularDish  *DishAnalytics `json:"most_popular_dish,omitempty"`
	BestRatedDish    *DishAnalytics `json:"best_rated_dish,omitempty"`
	MostPopularToday *DishAnalytics `json:"most_popular_today,omitempty"`
}
