package storage

import (
	"database/sql"
	"fmt"

	"overcooked-simplified/dish-svc/internal/domain"
)

type PostgresRepository struct {
	DB *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{DB: db}
}

func (r *PostgresRepository) CreateRestaurant(rest *domain.Restaurant) error {
	return r.DB.QueryRow(
		"INSERT INTO restaurants (name, address, description) VALUES ($1, $2, $3) RETURNING id, created_at",
		rest.Name, rest.Address, rest.Description,
	).Scan(&rest.ID, &rest.CreatedAt)
}

func (r *PostgresRepository) ListRestaurants() ([]domain.Restaurant, error) {
	rows, err := r.DB.Query(`
        SELECT id, name, COALESCE(address, ''), COALESCE(description, ''), COALESCE(image_url, ''), created_at
        FROM restaurants
        ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var restaurants []domain.Restaurant
	for rows.Next() {
		var rest domain.Restaurant
		if err := rows.Scan(&rest.ID, &rest.Name, &rest.Address, &rest.Description, &rest.ImageURL, &rest.CreatedAt); err != nil {
			continue
		}
		restaurants = append(restaurants, rest)
	}

	return restaurants, nil
}

func (r *PostgresRepository) GetRestaurant(id int) (*domain.Restaurant, error) {
	var rest domain.Restaurant
	err := r.DB.QueryRow(`
		SELECT id, name, COALESCE(address, ''), COALESCE(description, ''), COALESCE(image_url, ''), created_at
		FROM restaurants
		WHERE id = $1`, id).
		Scan(&rest.ID, &rest.Name, &rest.Address, &rest.Description, &rest.ImageURL, &rest.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &rest, nil
}

func (r *PostgresRepository) UpdateRestaurant(rest *domain.Restaurant) error {
	return r.DB.QueryRow(
		"UPDATE restaurants SET name=$1, address=$2, description=$3 WHERE id=$4 RETURNING id, name, address, description, COALESCE(image_url, ''), created_at",
		rest.Name, rest.Address, rest.Description, rest.ID).
		Scan(&rest.ID, &rest.Name, &rest.Address, &rest.Description, &rest.ImageURL, &rest.CreatedAt)
}

func (r *PostgresRepository) DeleteRestaurant(id int) (int64, error) {
	result, err := r.DB.Exec("DELETE FROM restaurants WHERE id=$1", id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *PostgresRepository) UpdateRestaurantImage(id int, imageURL string) error {
	_, err := r.DB.Exec("UPDATE restaurants SET image_url=$1 WHERE id=$2", imageURL, id)
	return err
}

func (r *PostgresRepository) CreateDish(dish *domain.Dish) error {
	return r.DB.QueryRow(
		"INSERT INTO dishes (restaurant_id, name, description, price, image_url) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at",
		dish.RestaurantID, dish.Name, dish.Description, dish.Price, dish.ImageURL).
		Scan(&dish.ID, &dish.CreatedAt)
}

func (r *PostgresRepository) ListDishes(restaurantID int) ([]domain.Dish, error) {
	rows, err := r.DB.Query(`
		SELECT id, restaurant_id, name, description, price, COALESCE(image_url, ''), created_at
		FROM dishes
		WHERE restaurant_id = $1
		ORDER BY created_at DESC`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dishes []domain.Dish
	for rows.Next() {
		var dish domain.Dish
		if err := rows.Scan(&dish.ID, &dish.RestaurantID, &dish.Name, &dish.Description, &dish.Price, &dish.ImageURL, &dish.CreatedAt); err != nil {
			continue
		}
		dishes = append(dishes, dish)
	}
	return dishes, nil
}

func (r *PostgresRepository) GetDish(restaurantID, dishID int) (*domain.Dish, error) {
	var dish domain.Dish
	err := r.DB.QueryRow(
		"SELECT id, restaurant_id, name, description, price, COALESCE(image_url, ''), created_at FROM dishes WHERE id = $1 AND restaurant_id = $2",
		dishID, restaurantID).
		Scan(&dish.ID, &dish.RestaurantID, &dish.Name, &dish.Description, &dish.Price, &dish.ImageURL, &dish.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &dish, nil
}

func (r *PostgresRepository) UpdateDish(dish *domain.Dish) error {
	_, err := r.DB.Exec(`
		UPDATE dishes
		SET name=$1, description=$2, price=$3
		WHERE id=$4 AND restaurant_id=$5`,
		dish.Name, dish.Description, dish.Price, dish.ID, dish.RestaurantID)
	return err
}

func (r *PostgresRepository) DeleteDish(restaurantID, dishID int) (int64, error) {
	result, err := r.DB.Exec("DELETE FROM dishes WHERE id=$1 AND restaurant_id=$2", dishID, restaurantID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *PostgresRepository) UpdateDishImage(restaurantID, dishID int, imageURL string) error {
	_, err := r.DB.Exec("UPDATE dishes SET image_url = $1 WHERE id = $2 AND restaurant_id = $3",
		imageURL, dishID, restaurantID)
	return err
}

func (r *PostgresRepository) CreateOrder(order *domain.Order) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := tx.QueryRow(`
		INSERT INTO orders (restaurant_id, total_amount, status, qr_code)
		VALUES ($1, $2, 'completed', NULL)
		RETURNING id, created_at
	`, order.RestaurantID, order.TotalAmount).Scan(&order.ID, &order.CreatedAt); err != nil {
		return err
	}

	for _, item := range order.Items {
		if _, err := tx.Exec(`
			INSERT INTO order_items (order_id, dish_id, quantity, price)
			VALUES ($1, $2, $3, $4)
		`, order.ID, item.DishID, item.Quantity, item.Price); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresRepository) SaveQRCode(orderID int, qr []byte) error {
	_, err := r.DB.Exec(`UPDATE orders SET qr_code = $1 WHERE id = $2`, qr, orderID)
	return err
}

func (r *PostgresRepository) GetOrder(orderID int) (*domain.Order, []domain.OrderItem, error) {
	var order domain.Order
	if err := r.DB.QueryRow(`
		SELECT id, restaurant_id, total_amount, status, created_at
		FROM orders WHERE id = $1
	`, orderID).Scan(&order.ID, &order.RestaurantID, &order.TotalAmount, &order.Status, &order.CreatedAt); err != nil {
		return nil, nil, err
	}

	var restaurantName string
	r.DB.QueryRow("SELECT name FROM restaurants WHERE id = $1", order.RestaurantID).Scan(&restaurantName)
	order.RestaurantName = restaurantName

	rows, err := r.DB.Query(`
		SELECT oi.dish_id, d.name, oi.quantity, oi.price
		FROM order_items oi
		JOIN dishes d ON oi.dish_id = d.id
		WHERE oi.order_id = $1
	`, orderID)
	if err != nil {
		return &order, nil, err
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.DishID, &item.DishName, &item.Quantity, &item.Price); err != nil {
			continue
		}
		items = append(items, item)
	}

	return &order, items, nil
}

func (r *PostgresRepository) ListOrders() ([]domain.Order, error) {
	rows, err := r.DB.Query(`
		SELECT id, restaurant_id, total_amount, status, created_at
		FROM orders
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(&order.ID, &order.RestaurantID, &order.TotalAmount, &order.Status, &order.CreatedAt); err != nil {
			continue
		}

		var restaurantName string
		r.DB.QueryRow("SELECT name FROM restaurants WHERE id = $1", order.RestaurantID).Scan(&restaurantName)
		order.RestaurantName = restaurantName

		orders = append(orders, order)
	}
	return orders, nil
}

func (r *PostgresRepository) GetQRCode(orderID int) ([]byte, error) {
	var qrCode []byte
	if err := r.DB.QueryRow("SELECT qr_code FROM orders WHERE id = $1", orderID).Scan(&qrCode); err != nil {
		return nil, err
	}
	return qrCode, nil
}

func (r *PostgresRepository) EnsureSchema() error {
	statements := []string{
		"ALTER TABLE IF EXISTS restaurants ADD COLUMN IF NOT EXISTS description TEXT",
		"ALTER TABLE IF EXISTS orders ADD COLUMN IF NOT EXISTS qr_code BYTEA",
	}
	for _, stmt := range statements {
		if _, err := r.DB.Exec(stmt); err != nil {
			return fmt.Errorf("ensure schema `%s`: %w", stmt, err)
		}
	}
	return nil
}
