package models

import "time"

type Achievement struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Image       string    `json:"image" validate:"required"`
	Title       string    `json:"title" validate:"required"`
	Description string    `json:"description" validate:"required"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
