package models

import "time"

type Product struct {
    ID              uint          `gorm:"primaryKey" json:"id"`
    Name            string        `json:"name" validate:"required"`
    Rating          float64       `json:"rating" validate:"required"`
    Quantity        uint          `json:"quantity" validate:"required"`
    Description     string        `json:"description" validate:"required"`
    Images          []string      `json:"images" gorm:"type:text;serializer:json"`
    Price           float64       `json:"price" validate:"required"`
    Info            string        `json:"info" validate:"required"`
    Feature         string        `json:"feature" validate:"required"`
    Guarantee       string        `json:"guarantee"`
    Discount        string        `json:"discount"`
    CreatedAt       time.Time     `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt       time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
    CategoryID      uint          `json:"category_id"`                         // Foreign key to Category
    BottomCategoryID uint         `json:"bottom_category_id"`                  // Foreign key to BottomCategory
    BrandID         uint          `json:"brand_id"`
    Category        Category      `gorm:"foreignKey:CategoryID" json:"category"`      // Belongs to one Category
    BottomCategory  BottomCategory `gorm:"foreignKey:BottomCategoryID" json:"bottom_category"` // Belongs to one BottomCategory
    Brand           Brand         `gorm:"foreignKey:BrandID" json:"brand"`
}
