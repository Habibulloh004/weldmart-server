package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"os"
	"weldmart/models"
)

var DB *gorm.DB

func InitDatabase() {
	var err error
	var dbPath string

	// Determine the database path based on the environment
	if os.Getenv("RENDER") == "true" {
		// Render environment (persistent disk path)
		dbPath = "/data/database.db"
	} else {
		// Local development (relative path in project root)
		dbPath = "database.db"
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("Database connected successfully at", dbPath)

	// Auto migrate the schema
	DB.AutoMigrate(
		&models.User{}, &models.Product{}, &models.Category{}, &models.Brand{},
		&models.Banner{}, &models.News{}, &models.Achievement{}, &models.Rassika{},
		&models.Order{}, &models.OrderItem{}, &models.HRassika{},
	)
}
