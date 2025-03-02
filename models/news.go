package models

import "time"

type News struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Image     string    `json:"image" validate:"required"`
	Text      string    `json:"text" validate:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
