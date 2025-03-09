package models

import (
	"time"
)

type Order struct {
	ID           uint        `gorm:"primaryKey" json:"id"`
	Price        float64     `json:"price" validate:"required"`
	Bonus        float64     `json:"bonus"`
	UserID       uint        `json:"user_id"`
	OrderType    string      `gorm:"column:order_type" json:"order_type"`
	Status       string      `json:"status"`
	Phone        string      `json:"phone,omitempty" gorm:"default:null"`
	Name         string      `json:"name,omitempty" gorm:"default:null"`
	Organization string      `json:"organization,omitempty" gorm:"default:null"`
	INN          string      `json:"inn,omitempty" gorm:"default:null"`
	Comment      string      `json:"comment,omitempty" gorm:"default:null"`
	CreatedAt    time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
	OrderItems   []OrderItem `gorm:"foreignKey:OrderID" json:"order_items"`
}

type OrderItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	OrderID   uint      `json:"order_id"`
	ProductID uint      `json:"product_id"`
	Quantity  int       `json:"quantity" validate:"required,min=1"`
	Product   Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
