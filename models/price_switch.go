package models

import "time"

// PriceSwitch model controls whether prices are shown in the application
type PriceSwitch struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Show      bool      `json:"show" gorm:"default:true"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
