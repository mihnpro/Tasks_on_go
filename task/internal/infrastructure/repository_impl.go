package infrastructure

import (
    "context"
    "errors"
    "online-store/internal/domain"
    "gorm.io/gorm"
)

type customerRepo struct {
    db *gorm.DB
}

func NewCustomerRepository(db *gorm.DB) domain.CustomerRepository {
    return &customerRepo{db: db}
}

func (r *customerRepo) GetByID(ctx context.Context, id uint) (*domain.Customer, error) {
    var c domain.Customer
    err := r.db.WithContext(ctx).First(&c, id).Error
    if err != nil {
        return nil, err
    }
    return &c, nil
}

func (r *customerRepo) UpdateEmail(ctx context.Context, id uint, email string) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        var count int64
        if err := tx.Model(&domain.Customer{}).
            Where("email = ? AND id != ?", email, id).
            Count(&count).Error; err != nil {
            return err
        }
        if count > 0 {
            return errors.New("email already in use")
        }
        return tx.Model(&domain.Customer{}).
            Where("id = ?", id).
            Update("email", email).Error
    })
}

type productRepo struct {
    db *gorm.DB
}

func NewProductRepository(db *gorm.DB) domain.ProductRepository {
    return &productRepo{db: db}
}

func (r *productRepo) Create(ctx context.Context, product *domain.Product) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return tx.Create(product).Error
    })
}

func (r *productRepo) GetByID(ctx context.Context, id uint) (*domain.Product, error) {
    var p domain.Product
    err := r.db.WithContext(ctx).First(&p, id).Error
    if err != nil {
        return nil, err
    }
    return &p, nil
}

type orderRepo struct {
    db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) domain.OrderRepository {
    return &orderRepo{db: db}
}

func (r *orderRepo) CreateOrderWithItems(ctx context.Context, order *domain.Order, items []domain.OrderItem) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Create(order).Error; err != nil {
            return err
        }
        var total float64 = 0
        for i := range items {
            items[i].OrderID = order.ID
            if err := tx.Create(&items[i]).Error; err != nil {
                return err
            }
            total += items[i].Subtotal
        }
        if err := tx.Model(order).Update("total_amount", total).Error; err != nil {
            return err
        }
        order.TotalAmount = total
        return nil
    })
}