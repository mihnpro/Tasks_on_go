package domain

import "context"

type CustomerRepository interface {
    GetByID(ctx context.Context, id uint) (*Customer, error)
    UpdateEmail(ctx context.Context, id uint, email string) error
}

type ProductRepository interface {
    Create(ctx context.Context, product *Product) error
    GetByID(ctx context.Context, id uint) (*Product, error)
}

type OrderRepository interface {
    CreateOrderWithItems(ctx context.Context, order *Order, items []OrderItem) error
}