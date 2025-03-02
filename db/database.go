package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"weldmart/models"
)

var DB *gorm.DB

func InitDatabase() {
	var err error
	DB, err = gorm.Open(sqlite.Open("database.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto migrate the schema
	DB.AutoMigrate(
		&models.User{}, &models.Product{}, &models.Category{}, &models.Brand{},
		&models.Banner{}, &models.News{}, &models.Achievement{}, &models.Rassika{}, &models.Order{}, &models.OrderItem{}, &models.HRassika{},
	)
}
