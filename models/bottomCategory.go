package models

import "time"

type BottomCategory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name" validate:"required"`
	Description string    `json:"description"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Image       string    `json:"image"`
	CategoryID  uint      `json:"category_id"`                                 // Foreign key to Category
	Category    Category  `gorm:"foreignKey:CategoryID" json:"category"`       // Belongs to one Category
	Products    []Product `gorm:"foreignKey:BottomCategoryID" json:"products"` // One-to-many with Product
}
