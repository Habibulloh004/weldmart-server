package db

import (
	"log"
	"os"
	"path/filepath"
	"weldmart/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDatabase() {
	var err error
	var dbPath string = "database.db"

	// Ensure the directory exists (create if it doesn't)
	dir := filepath.Dir(dbPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal("Failed to create database directory:", err)
		}
	}

	// Check if the database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Println("Database file does not exist, creating:", dbPath)
		// Create an empty database file if it doesn't exist
		file, err := os.Create(dbPath)
		if err != nil {
			log.Fatal("Failed to create database file:", err)
		}
		file.Close()
	}

	// Open the database (it will now exist or have been created)
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	log.Println("Database connected successfully at", dbPath)

	// Auto migrate the schema
	DB.AutoMigrate(
		&models.User{}, &models.Product{}, &models.Category{}, &models.Brand{},
		&models.Banner{}, &models.News{}, &models.Achievement{}, &models.Rassika{},
		&models.Order{}, &models.OrderItem{}, &models.HRassika{}, &models.Statistics{}, &models.Admin{}, &models.Clients{},
	)
}
