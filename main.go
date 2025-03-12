package main

import (
	"log"
	"os"

	"weldmart/db"
	"weldmart/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func main() {
	// Initialize database
	db.InitDatabase()

	// Create uploads directory if it doesn't exist
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		os.Mkdir("uploads", 0755)
	}

	// Create Fiber app
	app := fiber.New()

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Adjust for production
	}))

	// Serve static files
	app.Static("/uploads", "./uploads")

	// Setup routes (including WebSocket)
	routes.SetupRoutes(app)

	// Start Fiber server
	log.Println("Server starting on :8080")
	log.Fatal(app.Listen(":8080"))
}
