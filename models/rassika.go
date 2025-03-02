package models

import "time"

type Rassika struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `json:"email" validate:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	UserID    uint      `json:"user_id"`
}
