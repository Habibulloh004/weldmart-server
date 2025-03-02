package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Password  string    `json:"password"`
	Bonus     float64   `json:"bonus"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	RassikaID uint      `json:"rassika_id"`
	Orders    []Order   `gorm:"foreignKey:UserID" json:"orders"`
}
