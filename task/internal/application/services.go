package application

import (
	"context"
	"errors"
	"online-store/internal/domain"
	"time"
)

type OrderService struct {
	orderRepo   domain.OrderRepository
	productRepo domain.ProductRepository
	custRepo    domain.CustomerRepository
}

func NewOrderService(or domain.OrderRepository, pr domain.ProductRepository, cr domain.CustomerRepository) *OrderService {
	return &OrderService{
		orderRepo:   or,
		productRepo: pr,
		custRepo:    cr,
	}
}

type PlaceOrderRequest struct {
	CustomerID uint
	Items      []struct {
		ProductID uint
		Quantity  int
	}
}

func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*domain.Order, error) {
	// Проверяем существование клиента
	if _, err := s.custRepo.GetByID(ctx, req.CustomerID); err != nil {
		return nil, errors.New("customer not found")
	}

	// Подготавливаем позиции
	var items []domain.OrderItem
	for _, it := range req.Items {
		prod, err := s.productRepo.GetByID(ctx, it.ProductID)
		if err != nil {
			return nil, errors.New("product not found")
		}
		if it.Quantity <= 0 {
			return nil, errors.New("quantity must be positive")
		}
		items = append(items, domain.OrderItem{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			Subtotal:  prod.Price * float64(it.Quantity),
		})
	}

	order := &domain.Order{
		CustomerID:  req.CustomerID,
		OrderDate:   time.Now(),
		TotalAmount: 0,
	}

	err := s.orderRepo.CreateOrderWithItems(ctx, order, items)
	if err != nil {
		return nil, err
	}
	return order, nil
}

type CustomerService struct {
	repo domain.CustomerRepository
}

func NewCustomerService(repo domain.CustomerRepository) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) UpdateEmail(ctx context.Context, customerID uint, newEmail string) error {
	if newEmail == "" {
		return errors.New("email cannot be empty")
	}
	return s.repo.UpdateEmail(ctx, customerID, newEmail)
}

type ProductService struct {
	repo domain.ProductRepository
}

func NewProductService(repo domain.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) CreateProduct(ctx context.Context, name string, price float64) (*domain.Product, error) {
	if name == "" {
		return nil, errors.New("product name required")
	}
	if price < 0 {
		return nil, errors.New("price cannot be negative")
	}
	product := &domain.Product{
		Name:  name,
		Price: price,
	}
	err := s.repo.Create(ctx, product)
	if err != nil {
		return nil, err
	}
	return product, nil
}
