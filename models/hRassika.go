package models

import "time"

type HRassika struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string   `json:"title" validate:"required"`
	Body      string   `json:"body" validate:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
