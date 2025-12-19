package service

import (
	"errors"
	"fmt"

	"overcooked-simplified/dish-svc/internal/domain"
)

type RestaurantRepository interface {
	CreateRestaurant(rest *domain.Restaurant) error
	ListRestaurants() ([]domain.Restaurant, error)
	GetRestaurant(id int) (*domain.Restaurant, error)
	UpdateRestaurant(rest *domain.Restaurant) error
	DeleteRestaurant(id int) (int64, error)
	UpdateRestaurantImage(id int, imageURL string) error
}

type DishRepository interface {
	CreateDish(dish *domain.Dish) error
	ListDishes(restaurantID int) ([]domain.Dish, error)
	GetDish(restaurantID, dishID int) (*domain.Dish, error)
	UpdateDish(dish *domain.Dish) error
	DeleteDish(restaurantID, dishID int) (int64, error)
	UpdateDishImage(restaurantID, dishID int, imageURL string) error
}

type OrderRepository interface {
	CreateOrder(order *domain.Order) error
	SaveQRCode(orderID int, qr []byte) error
	GetOrder(orderID int) (*domain.Order, []domain.OrderItem, error)
	ListOrders() ([]domain.Order, error)
	GetQRCode(orderID int) ([]byte, error)
}

type RestaurantServiceInterface interface {
	Create(rest *domain.Restaurant) error
	List() ([]domain.Restaurant, error)
	Get(id int) (*domain.Restaurant, error)
	Update(rest *domain.Restaurant) error
	Delete(id int) (int64, error)
	UpdateImage(id int, imageURL string) error
}

type DishServiceInterface interface {
	Create(dish *domain.Dish) error
	List(restaurantID int) ([]domain.Dish, error)
	Get(restaurantID, dishID int) (*domain.Dish, error)
	Update(dish *domain.Dish) error
	Delete(restaurantID, dishID int) (int64, error)
	UpdateImage(restaurantID, dishID int, imageURL string) error
}

type OrderServiceInterface interface {
	Create(order *domain.Order) error
	SaveQRCode(orderID int, qr []byte) error
	Get(orderID int) (*domain.Order, error)
	List() ([]domain.Order, error)
	GetQRCode(orderID int) ([]byte, error)
	QRLink(orderID int) string
}

type RestaurantService struct {
	repo RestaurantRepository
}

func NewRestaurantService(repo RestaurantRepository) *RestaurantService {
	return &RestaurantService{repo: repo}
}

func (s *RestaurantService) Create(rest *domain.Restaurant) error {
	return s.repo.CreateRestaurant(rest)
}

func (s *RestaurantService) List() ([]domain.Restaurant, error) {
	return s.repo.ListRestaurants()
}

func (s *RestaurantService) Get(id int) (*domain.Restaurant, error) {
	return s.repo.GetRestaurant(id)
}

func (s *RestaurantService) Update(rest *domain.Restaurant) error {
	return s.repo.UpdateRestaurant(rest)
}

func (s *RestaurantService) Delete(id int) (int64, error) {
	return s.repo.DeleteRestaurant(id)
}

func (s *RestaurantService) UpdateImage(id int, imageURL string) error {
	return s.repo.UpdateRestaurantImage(id, imageURL)
}

var _ RestaurantServiceInterface = (*RestaurantService)(nil)

type DishService struct {
	repo DishRepository
}

func NewDishService(repo DishRepository) *DishService {
	return &DishService{repo: repo}
}

func (s *DishService) Create(dish *domain.Dish) error {
	return s.repo.CreateDish(dish)
}

func (s *DishService) List(restaurantID int) ([]domain.Dish, error) {
	return s.repo.ListDishes(restaurantID)
}

func (s *DishService) Get(restaurantID, dishID int) (*domain.Dish, error) {
	return s.repo.GetDish(restaurantID, dishID)
}

func (s *DishService) Update(dish *domain.Dish) error {
	return s.repo.UpdateDish(dish)
}

func (s *DishService) Delete(restaurantID, dishID int) (int64, error) {
	return s.repo.DeleteDish(restaurantID, dishID)
}

func (s *DishService) UpdateImage(restaurantID, dishID int, imageURL string) error {
	return s.repo.UpdateDishImage(restaurantID, dishID, imageURL)
}

var _ DishServiceInterface = (*DishService)(nil)

type OrderService struct {
	repo      OrderRepository
	qrEncoder QRGenerator
}

func NewOrderService(repo OrderRepository, qr QRGenerator) *OrderService {
	return &OrderService{repo: repo, qrEncoder: qr}
}

func (s *OrderService) Create(order *domain.Order) error {
	if order.RestaurantID <= 0 || len(order.Items) == 0 {
		return errors.New("invalid order payload")
	}
	if err := s.repo.CreateOrder(order); err != nil {
		return err
	}

	if s.qrEncoder != nil {
		if qr, err := s.qrEncoder.Generate(order.ID); err == nil {
			_ = s.repo.SaveQRCode(order.ID, qr)
		}
	}

	return nil
}

func (s *OrderService) SaveQRCode(orderID int, qr []byte) error {
	return s.repo.SaveQRCode(orderID, qr)
}

func (s *OrderService) Get(orderID int) (*domain.Order, error) {
	order, items, err := s.repo.GetOrder(orderID)
	if err != nil {
		return nil, err
	}
	order.Items = items
	return order, nil
}

func (s *OrderService) List() ([]domain.Order, error) {
	return s.repo.ListOrders()
}

func (s *OrderService) GetQRCode(orderID int) ([]byte, error) {
	qr, err := s.repo.GetQRCode(orderID)
	if err != nil {
		return nil, err
	}
	if len(qr) == 0 && s.qrEncoder != nil {
		if regenerated, err := s.qrEncoder.Generate(orderID); err == nil {
			_ = s.repo.SaveQRCode(orderID, regenerated)
			return regenerated, nil
		}
	}
	return qr, nil
}

func (s *OrderService) QRLink(orderID int) string {
	return fmt.Sprintf("/api/orders/%d/qrcode", orderID)
}

var _ OrderServiceInterface = (*OrderService)(nil)
