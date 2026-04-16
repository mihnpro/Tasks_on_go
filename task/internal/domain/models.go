package domain

import (
	"time"

	_ "gorm.io/gorm"
)

type Customer struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	FirstName string `gorm:"not null" json:"firstName"`
	LastName  string `gorm:"not null" json:"lastName"`
	Email     string `gorm:"uniqueIndex;not null" json:"email"`
}

type Product struct {
	ID    uint    `gorm:"primaryKey" json:"id"`
	Name  string  `gorm:"not null" json:"name"`
	Price float64 `gorm:"not null" json:"price"`
}

type Order struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	CustomerID  uint        `gorm:"not null;index" json:"customerId"`
	OrderDate   time.Time   `gorm:"not null" json:"orderDate"`
	TotalAmount float64     `gorm:"not null;default:0" json:"totalAmount"`
	Customer    Customer    `gorm:"foreignKey:CustomerID" json:"customer"`
	Items       []OrderItem `json:"items"`
}

type OrderItem struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	OrderID   uint    `gorm:"not null;index" json:"orderId"`
	ProductID uint    `gorm:"not null" json:"productId"`
	Quantity  int     `gorm:"not null" json:"quantity"`
	Subtotal  float64 `gorm:"not null" json:"subtotal"`
	Product   Product `gorm:"foreignKey:ProductID" json:"product"`
}
