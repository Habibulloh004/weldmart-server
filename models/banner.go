package models

import "time"

type Banner struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	URL       string    `json:"url" validate:"required"`
	Image     string    `json:"image" validate:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}