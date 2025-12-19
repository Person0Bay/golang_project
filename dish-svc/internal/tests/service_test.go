package tests

import (
	"overcooked-simplified/dish-svc/internal/domain"
	"overcooked-simplified/dish-svc/internal/mocks"
	"overcooked-simplified/dish-svc/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRestaurantService_Create(t *testing.T) {
	tests := []struct {
		name      string
		input     *domain.Restaurant
		mockError error
		wantErr   bool
	}{
		{
			name:    "valid restaurant",
			input:   &domain.Restaurant{Name: "Test", Address: "Addr", Description: "Desc"},
			wantErr: false,
		},
		{
			name:      "database error",
			input:     &domain.Restaurant{Name: "Test"},
			mockError: assert.AnError,
			wantErr:   true,
		},
		{
			name:      "empty name validation",
			input:     &domain.Restaurant{Name: ""},
			mockError: nil,
			wantErr:   false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := new(mocks.RestaurantRepository)
			svc := service.NewRestaurantService(mockRepo)

			mockRepo.On("CreateRestaurant", testCase.input).Return(testCase.mockError).Once()

			err := svc.Create(testCase.input)

			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestDishService_Get(t *testing.T) {
	tests := []struct {
		name      string
		restID    int
		dishID    int
		mockDish  *domain.Dish
		mockError error
		wantErr   bool
	}{
		{
			name:     "dish found",
			restID:   1,
			dishID:   1,
			mockDish: &domain.Dish{ID: 1, Name: "Pizza"},
			wantErr:  false,
		},
		{
			name:      "dish not found",
			restID:    1,
			dishID:    999,
			mockError: assert.AnError,
			wantErr:   true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := new(mocks.DishRepository)
			svc := service.NewDishService(mockRepo)

			if testCase.mockError != nil {
				mockRepo.On("GetDish", testCase.restID, testCase.dishID).Return(nil, testCase.mockError).Once()
			} else {
				mockRepo.On("GetDish", testCase.restID, testCase.dishID).Return(testCase.mockDish, nil).Once()
			}

			result, err := svc.Get(testCase.restID, testCase.dishID)

			if testCase.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.mockDish, result)
			}
		})
	}
}

func TestOrderService_CreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		order   *domain.Order
		wantErr bool
	}{
		{
			name:    "invalid: no restaurant ID",
			order:   &domain.Order{RestaurantID: 0, Items: []domain.OrderItem{{}}},
			wantErr: true,
		},
		{
			name:    "invalid: no items",
			order:   &domain.Order{RestaurantID: 1, Items: []domain.OrderItem{}},
			wantErr: true,
		},
		{
			name:    "valid order",
			order:   &domain.Order{RestaurantID: 1, Items: []domain.OrderItem{{DishID: 1}}},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockRepo := new(mocks.OrderRepository)
			mockQR := new(mocks.QRGenerator)
			svc := service.NewOrderService(mockRepo, mockQR)

			if !testCase.wantErr {
				mockRepo.On("CreateOrder", testCase.order).Return(nil)
				mockQR.On("Generate", mock.Anything).Return([]byte("qr"), nil)
				mockRepo.On("SaveQRCode", mock.Anything, mock.Anything).Return(nil)
			}

			err := svc.Create(testCase.order)

			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultQRGenerator(t *testing.T) {
	gen := &service.DefaultQRGenerator{BaseURL: "http://localhost"}
	qr, err := gen.Generate(123)

	assert.NoError(t, err)
	assert.NotEmpty(t, qr)
}
