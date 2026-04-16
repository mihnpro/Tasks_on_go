package domain

import (
    "time"
    _ "gorm.io/gorm"
)

type Customer struct {
    ID        uint   `gorm:"primaryKey"`
    FirstName string `gorm:"not null"`
    LastName  string `gorm:"not null"`
    Email     string `gorm:"uniqueIndex;not null"`
}

type Product struct {
    ID    uint    `gorm:"primaryKey"`
    Name  string  `gorm:"not null"`
    Price float64 `gorm:"not null"`
}

type Order struct {
    ID          uint      `gorm:"primaryKey"`
    CustomerID  uint      `gorm:"not null;index"`
    OrderDate   time.Time `gorm:"not null"`
    TotalAmount float64   `gorm:"not null;default:0"`
    Customer    Customer  `gorm:"foreignKey:CustomerID"`
    Items       []OrderItem
}

type OrderItem struct {
    ID        uint    `gorm:"primaryKey"`
    OrderID   uint    `gorm:"not null;index"`
    ProductID uint    `gorm:"not null"`
    Quantity  int     `gorm:"not null"`
    Subtotal  float64 `gorm:"not null"`
    Product   Product `gorm:"foreignKey:ProductID"`
}