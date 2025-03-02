package models

import "time"

type Brand struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name" validate:"required"`
	Country     string    `json:"country" validate:"required"`
	Description string    `json:"description" validate:"required"`
	Image       string    `json:"image" validate:"required"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Products    []Product `gorm:"foreignKey:BrandID" json:"products"`
}
