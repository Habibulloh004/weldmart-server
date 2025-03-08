package models

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name" gorm:"default:null"`
	Phone     string    `gorm:"unique" json:"phone"`
	Password  string    `json:"password"`
	Bonus     float64   `json:"bonus" gorm:"default:null"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	RassikaID uint      `json:"rassika_id" gorm:"default:null"`
	Orders    []Order   `gorm:"foreignKey:UserID; default:null" json:"orders"`
}