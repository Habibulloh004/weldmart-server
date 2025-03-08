package models

import "gorm.io/gorm"

// Statistics represents the single statistics object
type Statistics struct {
    gorm.Model
    ProductsCount string `json:"products" gorm:"not null"`
    PartnersCount string `json:"partners" gorm:"not null"`
    ClientsCount  string `json:"clients" gorm:"not null"`
}