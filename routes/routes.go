package routes

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"weldmart/db"
	"weldmart/models"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust this for production
	},
}

// Connected clients map with mutex for thread safety
var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan []byte, 100) // Buffered channel to prevent blocking
var mutex = &sync.Mutex{}
var validate = validator.New()

type OrderItemResponse struct {
	OrderQuantity int       `json:"order_quantity"`
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	Rating        float64   `json:"rating"`
	Quantity      uint      `json:"quantity"`
	Description   string    `json:"description"`
	Images        []string  `json:"images"`
	Price         float64   `json:"price"`
	Info          string    `json:"info"`
	Feature       string    `json:"feature"`
	Guarantee     string    `json:"guarantee"`
	Discount      string    `json:"discount"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CategoryID    uint      `json:"category_id"`
	BrandID       uint      `json:"brand_id"`
}

type OrderResponse struct {
	ID           uint                `json:"id"`
	Price        float64             `json:"price"`
	Bonus        float64             `json:"bonus"`
	UserID       uint                `json:"user_id"`
	OrderType    string              `json:"order_type"`
	Status       string              `json:"status"`
	Service      string              `json:"service_mode"`
	Phone        string              `json:"phone,omitempty"`
	Name         string              `json:"name,omitempty"`
	Organization string              `json:"organization,omitempty"`
	INN          string              `json:"inn,omitempty"`
	Comment      string              `json:"comment,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	OrderItems   []OrderItemResponse `json:"order_items"`
}

type ProductResponse struct {
	Products []models.Product `json:"products"`
	Total    int              `json:"total"`
	Skip     int              `json:"skip"`
	Limit    int              `json:"limit"`
}

// CategoryResponse struct to shape the API response with full product details
type CategoryResponse struct {
	Categories []struct {
		ID          uint             `json:"id"`
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Image       string           `json:"image"`
		CreatedAt   time.Time        `json:"created_at"`
		UpdatedAt   time.Time        `json:"updated_at"`
		Products    []models.Product `json:"products,omitempty"`
	} `json:"categories"`
	Total int `json:"total"`
	Skip  int `json:"skip"`
	Limit int `json:"limit"`
}

type LoginRequest struct {
	Phone    string `json:"phone" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse defines the structure of the login response
type LoginResponse struct {
	Message string      `json:"message"`
	User    models.User `json:"user"` // Full user details
}

type CategoryWithProductsResponse struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Image       string           `json:"image"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Total       int              `json:"total"` // Total products in this category
	Skip        int              `json:"skip"`  // Product skip
	Limit       int              `json:"limit"` // Product limit
	Products    []models.Product `json:"products"`
}

type BrandWithProductsResponse struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Country     string           `json:"country"`
	Description string           `json:"description"`
	Image       string           `json:"image"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Total       int              `json:"total"` // Total products in this brand
	Skip        int              `json:"skip"`  // Product skip
	Limit       int              `json:"limit"` // Product limit
	Products    []models.Product `json:"products"`
}

type BrandWithProducts struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Country     string           `json:"country"`
	Description string           `json:"description"`
	Image       string           `json:"image"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Products    []models.Product `json:"products,omitempty"`
}

type BrandListResponse struct {
	Brands []BrandWithProducts `json:"brands"`
	Total  int                 `json:"total"` // Total brands
	Skip   int                 `json:"skip"`  // Brand skip
	Limit  int                 `json:"limit"` // Brand limit
}

type SearchResponse struct {
	Products []models.Product `json:"products"`
}

func SetupRoutes(app *fiber.App) {

	wsHandler := adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Error upgrading:", err)
			return
		}
		defer conn.Close()

		mutex.Lock()
		clients[conn] = true
		mutex.Unlock()
		log.Println("Client connected:", conn.RemoteAddr())

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				mutex.Lock()
				delete(clients, conn)
				mutex.Unlock()
				log.Println("Client disconnected:", conn.RemoteAddr())
				break
			}
			log.Printf("Received message from %v: %s", conn.RemoteAddr(), string(message))
			broadcast <- message
		}
	})

	// Handle broadcasting messages to all clients
	go func() {
		for message := range broadcast {
			mutex.Lock()
			for client := range clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("WebSocket write error: %v", err)
					client.Close()
					delete(clients, client)
				}
			}
			mutex.Unlock()
		}
	}()

	// Mount WebSocket endpoint
	app.Get("/ws", wsHandler)
	// Image upload route
	app.Post("/upload", uploadImage)

	// User routes
	api := app.Group("/api")

	admin := api.Group("/admin")
	admin.Post("/", createAdmin)
	admin.Put("/", updateAdmin)
	admin.Get("/", getAdmin)

	api.Post("/login", loginHandler)

	users := api.Group("/users")
	users.Post("/", createUser)
	users.Get("/", getAllUsers)
	users.Get("/:id", getUser)
	users.Put("/:id", updateUser)
	users.Delete("/:id", deleteUser)

	stats := api.Group("/statistics")
	stats.Get("/", getStatistics)
	stats.Post("/", createStatistics)
	stats.Put("/", updateStatistics)

	// Product routes
	products := api.Group("/products")
	products.Get("/search", searchProducts)
	products.Post("/", createProduct)
	products.Get("/", getAllProducts)
	products.Get("/:id", getProduct)
	products.Put("/:id", updateProduct)
	products.Delete("/:id", deleteProduct)

	// Category routes
	categories := api.Group("/categories")
	categories.Post("/", createCategory)
	categories.Get("/", getAllCategories)
	categories.Get("/:id", getCategory)
	categories.Put("/:id", updateCategory)
	categories.Delete("/:id", deleteCategory)

	// Brand routes
	brands := api.Group("/brands")
	brands.Post("/", createBrand)
	brands.Get("/", getAllBrands)
	brands.Get("/:id", getBrand)
	brands.Put("/:id", updateBrand)
	brands.Delete("/:id", deleteBrand)

	// Banner routes
	banners := api.Group("/banners")
	banners.Post("/", createBanner)
	banners.Get("/", getAllBanners)
	banners.Get("/:id", getBanner)
	banners.Put("/:id", updateBanner)
	banners.Delete("/:id", deleteBanner)

	// News routes
	news := api.Group("/news")
	news.Post("/", createNews)
	news.Get("/", getAllNews)
	news.Get("/:id", getNewsItem)
	news.Put("/:id", updateNews)
	news.Delete("/:id", deleteNews)

	// Achievement routes
	achievements := api.Group("/achievements")
	achievements.Post("/", createAchievement)
	achievements.Get("/", getAllAchievements)
	achievements.Get("/:id", getAchievement)
	achievements.Put("/:id", updateAchievement)
	achievements.Delete("/:id", deleteAchievement)

	// Rassika routes
	rassikas := api.Group("/rassikas")
	rassikas.Post("/", createRassika)
	rassikas.Get("/", getAllRassikas)
	rassikas.Get("/:id", getRassika)
	rassikas.Put("/:id", updateRassika)
	rassikas.Delete("/:id", deleteRassika)

	hrassikas := api.Group("/hrassikas")
	hrassikas.Post("/", createHRassika)
	hrassikas.Get("/", getAllHRassika)
	hrassikas.Get("/:id", getHRassika)
	hrassikas.Put("/:id", updateHRassika)
	hrassikas.Delete("/:id", deleteHRassika)

	clients := api.Group("/clients")
	clients.Post("/", createClient)
	clients.Get("/", getAllClients)
	clients.Get("/:id", getClient)
	clients.Put("/:id", updateClient)
	clients.Delete("/:id", deleteClient)

	// Individual Order routes
	individualOrders := api.Group("/individual-orders")
	individualOrders.Post("/", createIndividualOrder)
	// individualOrders.Get("/", getAllIndividualOrders)
	// individualOrders.Get("/:id", getIndividualOrder)
	// individualOrders.Put("/:id", updateIndividualOrder)
	// individualOrders.Delete("/:id", deleteIndividualOrder)

	// Legal Order routes
	legalOrders := api.Group("/legal-orders")
	legalOrders.Post("/", createLegalOrder)
	// legalOrders.Get("/", getAllLegalOrders)
	// legalOrders.Get("/:id", getLegalOrder)
	// legalOrders.Put("/:id", updateLegalOrder)
	// legalOrders.Delete("/:id", deleteLegalOrder)

	// Order routes
	orders := api.Group("/orders")
	// orders.Post("/", createOrder)
	orders.Get("/", getAllOrders)
	orders.Get("/:id", getOrder)
	orders.Put("/:id", updateOrder)
	// orders.Put("/:id", updateOrder)
	orders.Delete("/:id", deleteOrder)
}

// Image upload handler
func uploadImage(c *fiber.Ctx) error {
	file, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to get uploaded file",
		})
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	uniqueID := uuid.New().String()
	filename := uniqueID + ext
	filepath := "./uploads/" + filename

	// Save the file
	if err := c.SaveFile(file, filepath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save file",
		})
	}

	// Return the file path that can be stored in the database
	return c.JSON(fiber.Map{
		"filename": filename,
		"path":     "/uploads/" + filename,
	})
}

func createClient(c *fiber.Ctx) error {
	client := new(models.Clients)

	// Parse request body
	if err := c.BodyParser(client); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Validate required fields
	validate := validator.New()
	if err := validate.Struct(client); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Image field is required",
		})
	}

	// Create client in database
	if err := db.DB.Create(&client).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create client",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(client)
}

// GetAllClients - GET /clients
func getAllClients(c *fiber.Ctx) error {
	var clients []models.Clients

	if err := db.DB.Find(&clients).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get clients",
		})
	}

	return c.JSON(clients)
}

// GetClient - GET /clients/:id
func getClient(c *fiber.Ctx) error {
	id := c.Params("id")
	var client models.Clients

	if err := db.DB.First(&client, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Client not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get client",
		})
	}

	return c.JSON(client)
}

// UpdateClient - PUT /clients/:id
func updateClient(c *fiber.Ctx) error {
	id := c.Params("id")
	client := new(models.Clients)

	// Parse request body
	if err := c.BodyParser(client); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Check if client exists
	var existingClient models.Clients
	if err := db.DB.First(&existingClient, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Client not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find client",
		})
	}

	// Validate required fields
	validate := validator.New()
	if err := validate.Struct(client); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Image field is required",
		})
	}

	// Update client
	if err := db.DB.Model(&existingClient).Updates(client).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update client",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Client updated successfully",
		"data":    existingClient,
	})
}

// DeleteClient - DELETE /clients/:id
func deleteClient(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if client exists
	var client models.Clients
	if err := db.DB.First(&client, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Client not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find client",
		})
	}

	// Delete client
	if err := db.DB.Delete(&client).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete client",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Client deleted successfully",
	})
}

func createAdmin(c *fiber.Ctx) error {
	var existingAdmin models.Admin
	if err := db.DB.First(&existingAdmin).Error; err == nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Admin already exists. Use PUT to update",
		})
	} else if err != gorm.ErrRecordNotFound {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error checking admin existence",
		})
	}

	admin := new(models.Admin)
	if err := c.BodyParser(admin); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if result := db.DB.Create(admin); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create admin",
		})
	}

	return c.JSON(admin)
}

func updateAdmin(c *fiber.Ctx) error {
	var existingAdmin models.Admin
	if err := db.DB.First(&existingAdmin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "No admin exists. Use POST to create",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error checking admin existence",
		})
	}

	admin := new(models.Admin)
	if err := c.BodyParser(admin); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	admin.ID = 1
	if result := db.DB.Model(&models.Admin{}).Where("id = ?", 1).Updates(admin); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update admin",
		})
	}

	return c.JSON(admin)
}

func getAdmin(c *fiber.Ctx) error {
	var admin models.Admin
	if err := db.DB.First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No admin exists",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch admin",
		})
	}

	return c.JSON(admin)
}

func loginHandler(c *fiber.Ctx) error {
	// Parse request body
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	// Validate required fields
	if req.Phone == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			" 하기error": "Phone and password are required",
		})
	}

	// Find user by phone number
	var user models.User
	if err := db.DB.Where("phone = ?", req.Phone).First(&user).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid phone number or password",
		})
	}

	// Compare plain text password
	if user.Password != req.Password {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid phone number or password",
		})
	}

	// Successful login
	response := LoginResponse{
		Message: "Login successful",
		User:    user, // Include full user struct (password excluded by json:"-")
	}

	return c.JSON(response)
}

func getStatistics(c *fiber.Ctx) error {
	var stats models.Statistics
	if err := db.DB.First(&stats, 1).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Statistics record not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve statistics",
		})
	}
	return c.JSON(stats)
}

func createStatistics(c *fiber.Ctx) error {
	var stats models.Statistics
	// Check if the record exists (ID=1)
	if err := db.DB.First(&stats, 1).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check statistics record",
			})
		}
		// If not found, create the record
		var newStats models.Statistics
		if err := c.BodyParser(&newStats); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Failed to parse request body",
			})
		}

		// Validate the incoming data
		if err := validate.Struct(&newStats); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Create the record
		if err := db.DB.Create(&newStats).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to create statistics",
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"success": true,
			"message": "Statistics created successfully",
		})
	}

	// If the record exists, update it instead
	var updateData models.Statistics
	if err := c.BodyParser(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Validate the incoming data
	if err := validate.Struct(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Update the existing record
	db.DB.Model(&stats).Updates(updateData)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Statistics updated successfully (record already existed)",
	})
}

func updateStatistics(c *fiber.Ctx) error {
	var stats models.Statistics
	if err := db.DB.First(&stats, 1).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Statistics record not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve statistics",
		})
	}

	// Parse the incoming update data
	var updateData models.Statistics
	if err := c.BodyParser(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Validate the incoming data
	if err := validate.Struct(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Update achievement (single record with ID=1)
	db.DB.Model(&stats).Updates(updateData)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Statistics updated successfully",
	})
}

// User handlers
func createUser(c *fiber.Ctx) error {
	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Validate phone format
	phoneRegex := regexp.MustCompile(`^\+\d{12}$`) // Adjusted for your 12-digit example
	if !phoneRegex.MatchString(user.Phone) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Phone number must be 10-13 digits",
		})
	}

	// Log phone check time
	var existingUser models.User
	if err := db.DB.Where("phone = ?", user.Phone).First(&existingUser).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to check phone number",
			})
		}
	} else {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Phone number already in use",
		})
	}

	// Log insert time
	if err := db.DB.Create(&user).Error; err != nil {
		if gorm.ErrDuplicatedKey == err {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Phone number already in use",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

func getAllUsers(c *fiber.Ctx) error {
	var users []models.User
	// Preload Orders, OrderItems, and Product (with Category and Brand)
	if err := db.DB.
		Preload("Orders.OrderItems.Product.Category").
		Preload("Orders.OrderItems.Product.Brand").
		Preload("Orders.OrderItems.Product").
		Preload("Orders.OrderItems").
		Preload("Orders").
		Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get users",
		})
	}

	return c.JSON(users)
}

func getUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User

	// Preload Orders, OrderItems, and Product (with Category and Brand)
	if err := db.DB.
		Preload("Orders.OrderItems.Product.Category").
		Preload("Orders.OrderItems.Product.Brand").
		Preload("Orders.OrderItems.Product").
		Preload("Orders.OrderItems").
		Preload("Orders").
		First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(user)
}

func updateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	user := new(models.User)

	if err := c.BodyParser(user); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	var existingUser models.User
	if err := db.DB.First(&existingUser, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	// Validate phone number if provided
	if user.Phone != "" {
		phoneRegex := regexp.MustCompile(`^\+\d{12}$`)
		if !phoneRegex.MatchString(user.Phone) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Phone number must be 13 digits",
			})
		}
		var conflictingUser models.User
		if err := db.DB.Where("phone = ? AND id != ?", user.Phone, id).First(&conflictingUser).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to check phone number",
				})
			}
		} else {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Phone number already in use by another user",
			})
		}
	}

	if err := db.DB.Model(&existingUser).Updates(user).Error; err != nil {
		if gorm.ErrDuplicatedKey == err {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Phone number already in use",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User updated successfully",
	})
}

func deleteUser(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.User{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete user",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User deleted successfully",
	})
}

// Product handlers
func createProduct(c *fiber.Ctx) error {
	product := new(models.Product)
	if err := c.BodyParser(product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Validate if the CategoryID exists if provided
	if product.CategoryID != 0 {
		var category models.Category
		if err := db.DB.First(&category, product.CategoryID).Error; err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Category not found",
			})
		}
	}

	if err := db.DB.Create(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create product",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

func searchProducts(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query parameter 'q' is required",
		})
	}

	var products []models.Product

	// Step 1: Search by Product Name
	if err := db.DB.Preload("Category").Preload("Brand").
		Where("name LIKE ?", "%"+query+"%").Find(&products).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search products",
		})
	}

	// If products are found by name, return them
	if len(products) > 0 {
		return c.JSON(SearchResponse{Products: products})
	}

	// Step 2: Search by Category Name
	var categoryIDs []uint
	if err := db.DB.Model(&models.Category{}).
		Where("name LIKE ?", "%"+query+"%").
		Pluck("id", &categoryIDs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search categories",
		})
	}

	if len(categoryIDs) > 0 {
		if err := db.DB.Preload("Category").Preload("Brand").
			Where("category_id IN ?", categoryIDs).Find(&products).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get products by category",
			})
		}
		// If products are found by category, return them
		if len(products) > 0 {
			return c.JSON(SearchResponse{Products: products})
		}
	}

	// Step 3: Search by Brand Name
	var brandIDs []uint
	if err := db.DB.Model(&models.Brand{}).
		Where("name LIKE ?", "%"+query+"%").
		Pluck("id", &brandIDs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to search brands",
		})
	}

	if len(brandIDs) > 0 {
		if err := db.DB.Preload("Category").Preload("Brand").
			Where("brand_id IN ?", brandIDs).Find(&products).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to get products by brand",
			})
		}
	}

	// Return the products (could be empty if no matches found)
	return c.JSON(SearchResponse{Products: products})
}

// GetAllProducts
func getAllProducts(c *fiber.Ctx) error {
	var total int64
	var products []models.Product

	// Get query parameters with error handling
	limitStr := c.Query("limit")
	skipStr := c.Query("skip")
	categoryID := c.Query("category_id")
	brandID := c.Query("brand_id") // New query parameter for brand_id

	var limit, skip int

	// Default values
	limit = -1 // No limit unless specified
	skip = 0   // Default skip to 0

	// Parse limit if provided
	if limitStr != "" {
		limit = c.QueryInt("limit", 0)
		if limit < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid limit parameter",
			})
		}
	}

	// Parse skip if provided
	if skipStr != "" {
		skip = c.QueryInt("skip", 0)
		if skip < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid skip parameter",
			})
		}
	}

	// Base query with preloading
	dbQuery := db.DB.Preload("Category").Preload("Brand")

	// Apply filters if provided
	if categoryID != "" {
		dbQuery = dbQuery.Where("category_id = ?", categoryID)
	}
	if brandID != "" {
		dbQuery = dbQuery.Where("brand_id = ?", brandID)
	}

	// Count total products (filtered by category_id and/or brand_id if applicable)
	if err := dbQuery.Model(&models.Product{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count products",
		})
	}

	// Apply pagination
	if skip > 0 {
		dbQuery = dbQuery.Offset(skip)
	}
	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(int(total)) // Fetch all after skip
	}

	// Fetch products
	if err := dbQuery.Find(&products).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get products",
		})
	}

	// Prepare response
	response := ProductResponse{
		Products: products,
		Total:    int(total),
		Skip:     skip,
		Limit:    limit,
	}

	return c.JSON(response)
}

// GetProduct
func getProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	var product models.Product

	// Preload full Category and Brand structs
	if err := db.DB.Preload("Category").Preload("Brand").First(&product, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Product not found",
		})
	}

	return c.JSON(product)
}

// UpdateProduct
func updateProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	product := new(models.Product)

	if err := c.BodyParser(product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	var existingProduct models.Product
	if err := db.DB.First(&existingProduct, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Product not found",
		})
	}

	// Validate if the CategoryID exists if provided
	if product.CategoryID != 0 {
		var category models.Category
		if err := db.DB.First(&category, product.CategoryID).Error; err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Category not found",
			})
		}
	}

	db.DB.Model(&models.Product{}).Where("id = ?", id).Updates(product)
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Product updated successfully",
	})
}

// DeleteProduct
func deleteProduct(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.Product{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete product",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Product deleted successfully",
	})
}

func createCategory(c *fiber.Ctx) error {
	category := new(models.Category)
	if err := c.BodyParser(category); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Ensure Products field is empty when creating a new category
	category.Products = nil

	if err := db.DB.Create(&category).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create category",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(category)
}

func getAllCategories(c *fiber.Ctx) error {
	var total int64
	var categories []models.Category

	// Get query parameters with error handling
	limitStr := c.Query("limit") // Get raw string value to check if it exists
	skipStr := c.Query("skip")   // Get raw string value to check if it exists

	var limit, skip int
	var err error

	// Default values (no limit or skip unless specified)
	limit = -1 // Use -1 to indicate no limit
	skip = 0   // Default skip to 0

	// Parse limit if it exists
	if limitStr != "" {
		limit = c.QueryInt("limit", 0) // Use 0 as default for parsing, we'll handle logic later
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid limit parameter",
			})
		}
	}

	// Parse skip if it exists
	if skipStr != "" {
		skip = c.QueryInt("skip", 0)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid skip parameter",
			})
		}
	}

	// Count total categories
	if err := db.DB.Model(&models.Category{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count categories",
		})
	}

	// Query with pagination and preload all product fields
	dbQuery := db.DB.Preload("Products")
	if skip > 0 {
		dbQuery = dbQuery.Offset(skip)
	}
	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	} else {
		// No limit specified, get all remaining items after skip
		dbQuery = dbQuery.Limit(int(total)) // Use total as a large limit to get all
	}

	if err := dbQuery.Find(&categories).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get categories",
		})
	}

	// Prepare response
	response := CategoryResponse{
		Categories: make([]struct {
			ID          uint             `json:"id"`
			Name        string           `json:"name"`
			Description string           `json:"description"`
			Image       string           `json:"image"`
			CreatedAt   time.Time        `json:"created_at"`
			UpdatedAt   time.Time        `json:"updated_at"`
			Products    []models.Product `json:"products,omitempty"`
		}, len(categories)),
		Total: int(total),
		Skip:  skip,
		Limit: limit,
	}

	// Map categories to response structure
	for i, category := range categories {
		response.Categories[i] = struct {
			ID          uint             `json:"id"`
			Name        string           `json:"name"`
			Description string           `json:"description"`
			Image       string           `json:"image"`
			CreatedAt   time.Time        `json:"created_at"`
			UpdatedAt   time.Time        `json:"updated_at"`
			Products    []models.Product `json:"products,omitempty"`
		}{
			ID:          category.ID,
			Name:        category.Name,
			Description: category.Description,
			Image:       category.Image,
			CreatedAt:   category.CreatedAt,
			UpdatedAt:   category.UpdatedAt,
			Products:    category.Products,
		}
	}

	return c.JSON(response)
}

// GetCategory - Already preloads Products
func getCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	var category models.Category
	var totalProducts int64

	// Get query parameters for product pagination
	productLimitStr := c.Query("product_limit")
	productSkipStr := c.Query("product_skip")

	var productLimit, productSkip int

	// Default values for product pagination
	productLimit = -1 // No limit unless specified
	productSkip = 0   // Default skip to 0

	// Parse product limit
	if productLimitStr != "" {
		productLimit = c.QueryInt("product_limit", 0)
		if productLimit < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid product_limit parameter",
			})
		}
	}

	// Parse product skip
	if productSkipStr != "" {
		productSkip = c.QueryInt("product_skip", 0)
		if productSkip < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid product_skip parameter",
			})
		}
	}

	// Count total products for this category
	if err := db.DB.Model(&models.Product{}).Where("category_id = ?", id).Count(&totalProducts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count products",
		})
	}

	// Query category with paginated products
	productQuery := db.DB.Model(&models.Product{}).Where("category_id = ?", id)
	if productSkip > 0 {
		productQuery = productQuery.Offset(productSkip)
	}
	if productLimit > 0 {
		productQuery = productQuery.Limit(productLimit)
	} else {
		productQuery = productQuery.Limit(int(totalProducts)) // Fetch all if no limit
	}

	if err := db.DB.Preload("Products", func(db *gorm.DB) *gorm.DB {
		return productQuery
	}).First(&category, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Category not found",
		})
	}

	// Prepare response
	response := CategoryWithProductsResponse{
		ID:          category.ID,
		Name:        category.Name,
		Description: category.Description,
		Image:       category.Image,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
		Total:       int(totalProducts),
		Skip:        productSkip,
		Limit:       productLimit,
		Products:    category.Products,
	}

	return c.JSON(response)
}

// UpdateCategory
func updateCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	category := new(models.Category)

	if err := c.BodyParser(category); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	var existingCategory models.Category
	if err := db.DB.First(&existingCategory, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Category not found",
		})
	}

	// Ensure Products field is not modified directly through updates
	category.Products = nil

	db.DB.Model(&models.Category{}).Where("id = ?", id).Updates(category)
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Category updated successfully",
	})
}

// DeleteCategory
func deleteCategory(c *fiber.Ctx) error {
	id := c.Params("id")

	// First, set category_id to NULL for all products in this category
	if err := db.DB.Model(&models.Product{}).Where("category_id = ?", id).Update("category_id", nil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update products",
		})
	}

	// Then delete the category
	if err := db.DB.Delete(&models.Category{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete category",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Category deleted successfully",
	})
}

// Brand handlers
func createBrand(c *fiber.Ctx) error {
	brand := new(models.Brand)
	if err := c.BodyParser(brand); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Ensure Products field is empty when creating a new brand
	brand.Products = nil

	if err := db.DB.Create(&brand).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create brand",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(brand)
}

func getAllBrands(c *fiber.Ctx) error {
	var total int64
	var brands []models.Brand

	// Get query parameters for brand pagination only
	limitStr := c.Query("limit")
	skipStr := c.Query("skip")

	var limit, skip int

	// Default values for brand pagination
	limit = -1 // No limit unless specified
	skip = 0   // Default skip to 0

	// Parse brand limit
	if limitStr != "" {
		limit = c.QueryInt("limit", 0)
		if limit < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid limit parameter",
			})
		}
	}

	// Parse brand skip
	if skipStr != "" {
		skip = c.QueryInt("skip", 0)
		if skip < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid skip parameter",
			})
		}
	}

	// Count total brands
	if err := db.DB.Model(&models.Brand{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count brands",
		})
	}

	// Query brands with pagination (only for brands)
	dbQuery := db.DB.Preload("Products") // Fetch all products without pagination
	if skip > 0 {
		dbQuery = dbQuery.Offset(skip)
	}
	if limit > 0 {
		dbQuery = dbQuery.Limit(limit)
	} else {
		dbQuery = dbQuery.Limit(int(total)) // Fetch all brands after skip
	}

	if err := dbQuery.Find(&brands).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get brands",
		})
	}

	// Prepare response
	response := BrandListResponse{
		Brands: make([]BrandWithProducts, len(brands)),
		Total:  int(total),
		Skip:   skip,
		Limit:  limit,
	}

	// Map brands to response structure
	for i, brand := range brands {
		response.Brands[i] = BrandWithProducts{
			ID:          brand.ID,
			Name:        brand.Name,
			Country:     brand.Country,
			Description: brand.Description,
			Image:       brand.Image,
			CreatedAt:   brand.CreatedAt,
			UpdatedAt:   brand.UpdatedAt,
			Products:    brand.Products, // All products for this brand
		}
	}

	return c.JSON(response)
}

func getBrand(c *fiber.Ctx) error {
	id := c.Params("id")
	var brand models.Brand
	var totalProducts int64

	// Get query parameters for product pagination
	productLimitStr := c.Query("product_limit")
	productSkipStr := c.Query("product_skip")

	var productLimit, productSkip int

	// Default values for product pagination
	productLimit = -1 // No limit unless specified
	productSkip = 0   // Default skip to 0

	// Parse product limit
	if productLimitStr != "" {
		productLimit = c.QueryInt("product_limit", 0)
		if productLimit < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid product_limit parameter",
			})
		}
	}

	// Parse product skip
	if productSkipStr != "" {
		productSkip = c.QueryInt("product_skip", 0)
		if productSkip < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid product_skip parameter",
			})
		}
	}

	// Count total products for this brand
	if err := db.DB.Model(&models.Product{}).Where("brand_id = ?", id).Count(&totalProducts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count products",
		})
	}

	// Query brand with paginated products
	productQuery := db.DB.Model(&models.Product{}).Where("brand_id = ?", id)
	if productSkip > 0 {
		productQuery = productQuery.Offset(productSkip)
	}
	if productLimit > 0 {
		productQuery = productQuery.Limit(productLimit)
	} else {
		productQuery = productQuery.Limit(int(totalProducts)) // Fetch all if no limit
	}

	if err := db.DB.Preload("Products", func(db *gorm.DB) *gorm.DB {
		return productQuery
	}).First(&brand, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Brand not found",
		})
	}

	// Prepare response
	response := BrandWithProductsResponse{
		ID:          brand.ID,
		Name:        brand.Name,
		Country:     brand.Country,
		Description: brand.Description,
		Image:       brand.Image,
		CreatedAt:   brand.CreatedAt,
		UpdatedAt:   brand.UpdatedAt,
		Total:       int(totalProducts),
		Skip:        productSkip,
		Limit:       productLimit,
		Products:    brand.Products,
	}

	return c.JSON(response)
}

func updateBrand(c *fiber.Ctx) error {
	id := c.Params("id")
	brand := new(models.Brand)

	if err := c.BodyParser(brand); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	var existingBrand models.Brand
	if err := db.DB.First(&existingBrand, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Brand not found",
		})
	}

	// Ensure Products field is not modified directly through updates
	brand.Products = nil

	// Update brand
	db.DB.Model(&models.Brand{}).Where("id = ?", id).Updates(brand)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Brand updated successfully",
	})
}

// DeleteBrand
func deleteBrand(c *fiber.Ctx) error {
	id := c.Params("id")

	// First, set brand_id to NULL for all products in this brand
	if err := db.DB.Model(&models.Product{}).Where("brand_id = ?", id).Update("brand_id", nil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update products",
		})
	}

	// Then delete the brand
	if err := db.DB.Delete(&models.Brand{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete brand",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Brand deleted successfully",
	})
}

// Banner handlers
func createBanner(c *fiber.Ctx) error {
	banner := new(models.Banner)
	if err := c.BodyParser(banner); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	if err := db.DB.Create(&banner).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create banner",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(banner)
}

func getAllBanners(c *fiber.Ctx) error {
	var banners []models.Banner
	if err := db.DB.Find(&banners).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get banners",
		})
	}

	return c.JSON(banners)
}

func getBanner(c *fiber.Ctx) error {
	id := c.Params("id")
	var banner models.Banner

	if err := db.DB.First(&banner, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Banner not found",
		})
	}

	return c.JSON(banner)
}

func updateBanner(c *fiber.Ctx) error {
	id := c.Params("id")
	banner := new(models.Banner)

	if err := c.BodyParser(banner); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Check if the banner exists
	if err := db.DB.First(&models.Banner{}, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Banner not found",
		})
	}

	// Update banner
	db.DB.Model(&models.Banner{}).Where("id = ?", id).Updates(banner)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Banner updated successfully",
	})
}

func deleteBanner(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.Banner{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete banner",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Banner deleted successfully",
	})
}

// News handlers
func createNews(c *fiber.Ctx) error {
	news := new(models.News)
	if err := c.BodyParser(news); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	if err := db.DB.Create(&news).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create news item",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(news)
}

func getAllNews(c *fiber.Ctx) error {
	var news []models.News
	if err := db.DB.Find(&news).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get news items",
		})
	}

	return c.JSON(news)
}

func getNewsItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var news models.News

	if err := db.DB.First(&news, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "News item not found",
		})
	}

	return c.JSON(news)
}

func updateNews(c *fiber.Ctx) error {
	id := c.Params("id")
	news := new(models.News)

	if err := c.BodyParser(news); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Check if the news item exists
	if err := db.DB.First(&models.News{}, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "News item not found",
		})
	}

	// Update news item
	db.DB.Model(&models.News{}).Where("id = ?", id).Updates(news)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "News item updated successfully",
	})
}

func deleteNews(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.News{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete news item",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "News item deleted successfully",
	})
}

// Achievement handlers
func createAchievement(c *fiber.Ctx) error {
	achievement := new(models.Achievement)
	if err := c.BodyParser(achievement); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	if err := db.DB.Create(&achievement).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create achievement",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(achievement)
}

func getAllAchievements(c *fiber.Ctx) error {
	var achievements []models.Achievement
	if err := db.DB.Find(&achievements).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get achievements",
		})
	}

	return c.JSON(achievements)
}

func getAchievement(c *fiber.Ctx) error {
	id := c.Params("id")
	var achievement models.Achievement

	if err := db.DB.First(&achievement, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Achievement not found",
		})
	}

	return c.JSON(achievement)
}

func updateAchievement(c *fiber.Ctx) error {
	id := c.Params("id")
	achievement := new(models.Achievement)

	if err := c.BodyParser(achievement); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Check if the achievement exists
	if err := db.DB.First(&models.Achievement{}, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Achievement not found",
		})
	}

	// Update achievement
	db.DB.Model(&models.Achievement{}).Where("id = ?", id).Updates(achievement)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Achievement updated successfully",
	})
}

func deleteAchievement(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.Achievement{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete achievement",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Achievement deleted successfully",
	})
}

// Rassika handlers
func createRassika(c *fiber.Ctx) error {
	rassika := new(models.Rassika)
	if err := c.BodyParser(rassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	if rassika.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	if rassika.UserID != nil {
		var user models.User
		if err := db.DB.First(&user, *rassika.UserID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Provided user_id does not exist",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify user_id",
			})
		}
	}

	if err := db.DB.Create(&rassika).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create rassika",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(rassika)
}

// main.go continued...

func getAllRassikas(c *fiber.Ctx) error {
	var rassikas []models.Rassika
	if err := db.DB.Find(&rassikas).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get rassikas",
		})
	}

	return c.JSON(rassikas)
}

func getRassika(c *fiber.Ctx) error {
	id := c.Params("id")
	var rassika models.Rassika

	// Remove Preload since there's no Users relation
	if err := db.DB.First(&rassika, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Rassika not found",
		})
	}

	return c.JSON(rassika)
}

func updateRassika(c *fiber.Ctx) error {
	id := c.Params("id")
	rassika := new(models.Rassika)

	if err := c.BodyParser(rassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body",
		})
	}

	// Check if the rassika exists
	var existingRassika models.Rassika
	if err := db.DB.First(&existingRassika, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Rassika not found",
		})
	}

	// Check if UserID is provided and valid
	if rassika.UserID != nil {
		var user models.User
		if err := db.DB.First(&user, *rassika.UserID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Provided user_id does not exist",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to verify user_id",
			})
		}
	}

	// Update rassika
	if err := db.DB.Model(&existingRassika).Updates(rassika).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update rassika",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Rassika updated successfully",
	})
}

func deleteRassika(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := db.DB.Delete(&models.Rassika{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete rassika",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Rassika deleted successfully",
	})
}

func createHRassika(c *fiber.Ctx) error {
	hrassika := new(models.HRassika)
	if err := c.BodyParser(hrassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body: " + err.Error(),
		})
	}

	// Validate the struct
	if err := validate.Struct(hrassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// Create the HRassika item in database
	if err := db.DB.Create(&hrassika).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create HRassika item: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(hrassika)
}

// GetAllHRassika retrieves all HRassika items
func getAllHRassika(c *fiber.Ctx) error {
	var hrassikas []models.HRassika
	if err := db.DB.Find(&hrassikas).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get HRassika items: " + err.Error(),
		})
	}

	return c.JSON(hrassikas)
}

// GetHRassika retrieves a single HRassika item by ID
func getHRassika(c *fiber.Ctx) error {
	id := c.Params("id")
	var hrassika models.HRassika

	if err := db.DB.First(&hrassika, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "HRassika item not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get HRassika item: " + err.Error(),
		})
	}

	return c.JSON(hrassika)
}

// UpdateHRassika updates an existing HRassika item
func updateHRassika(c *fiber.Ctx) error {
	id := c.Params("id")
	hrassika := new(models.HRassika)

	if err := c.BodyParser(hrassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body: " + err.Error(),
		})
	}

	// Validate the struct
	if err := validate.Struct(hrassika); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// Check if the HRassika item exists
	var existing models.HRassika
	if err := db.DB.First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "HRassika item not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find HRassika item: " + err.Error(),
		})
	}

	// Update only the provided fields
	result := db.DB.Model(&existing).Updates(models.HRassika{
		Title: hrassika.Title,
		Body:  hrassika.Body,
	})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update HRassika item: " + result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No changes made to HRassika item",
		})
	}

	// Return the updated item
	if err := db.DB.First(&existing, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch updated HRassika item",
		})
	}

	return c.JSON(existing)
}

// DeleteHRassika deletes an HRassika item by ID
func deleteHRassika(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if the HRassika item exists first
	var hrassika models.HRassika
	if err := db.DB.First(&hrassika, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "HRassika item not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to find HRassika item: " + err.Error(),
		})
	}

	// Delete the HRassika item
	result := db.DB.Delete(&models.HRassika{}, id)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete HRassika item: " + result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "HRassika item not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "HRassika item deleted successfully",
	})
}

func createIndividualOrder(c *fiber.Ctx) error {
	type IndividualOrderRequest struct {
		Price      float64 `json:"price" validate:"required,gte=0"`
		Bonus      float64 `json:"bonus" validate:"gte=0"`
		UserID     uint    `json:"user_id"`
		Status     string  `json:"status" validate:"required"`
		Service    string  `json:"service_mode" validate:"required"`
		Phone      string  `json:"phone" validate:"required"`
		Name       string  `json:"name" validate:"required"`
		Comment    string  `json:"comment"`
		OrderItems []struct {
			ProductID uint `json:"product_id" validate:"required"`
			Quantity  int  `json:"quantity" validate:"required,gte=1"`
		} `json:"order_items" validate:"required,dive"`
	}

	var requestData IndividualOrderRequest
	if err := c.BodyParser(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body: " + err.Error(),
		})
	}

	if err := validate.Struct(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// var user models.User
	// if err := db.DB.First(&user, requestData.UserID).Error; err != nil {
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"error": "User not found",
	// 	})
	// }

	order := models.Order{
		Price:     requestData.Price,
		Bonus:     requestData.Bonus,
		UserID:    requestData.UserID,
		Status:    requestData.Status,
		Service:   requestData.Service,
		OrderType: "individual",
		Phone:     requestData.Phone,
		Name:      requestData.Name,
		Comment:   requestData.Comment,
	}

	tx := db.DB.Begin()
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create individual order: " + err.Error(),
		})
	}

	var orderItems []models.OrderItem
	var calculatedPrice float64
	for _, item := range requestData.OrderItems {
		var product models.Product
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Product %d not found", item.ProductID),
			})
		}

		if uint(item.Quantity) > product.Quantity {
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Insufficient quantity for product %d", item.ProductID),
			})
		}

		orderItems = append(orderItems, models.OrderItem{
			OrderID:   order.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
		calculatedPrice += product.Price * float64(item.Quantity)
	}

	// if calculatedPrice != requestData.Price {
	// 	tx.Rollback()
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"error":    "Price doesn't match order items total",
	// 		"expected": calculatedPrice,
	// 		"received": requestData.Price,
	// 	})
	// }

	if err := tx.Create(&orderItems).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create order items: " + err.Error(),
		})
	}

	for _, item := range orderItems {
		if err := tx.Model(&models.Product{}).
			Where("id = ?", item.ProductID).
			Update("quantity", gorm.Expr("quantity - ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update product quantities",
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction",
		})
	}

	// Verify the association
	var checkUser models.User
	if err := db.DB.Preload("Orders").First(&checkUser, requestData.UserID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Order created but failed to verify user association",
		})
	}
	fmt.Printf("User orders after creation: %+v\n", checkUser.Orders) // Debug log

	var fullOrder models.Order
	if err := db.DB.Preload("OrderItems.Product").First(&fullOrder, order.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Order created but failed to load full details",
		})
	}

	orderResponse := OrderResponse{
		ID:        fullOrder.ID,
		Price:     fullOrder.Price,
		Bonus:     fullOrder.Bonus,
		UserID:    fullOrder.UserID,
		Status:    fullOrder.Status,
		Service:   fullOrder.Service,
		OrderType: fullOrder.OrderType,
		Comment:   fullOrder.Comment,
		Phone:     fullOrder.Phone,
		Name:      fullOrder.Name,
		CreatedAt: fullOrder.CreatedAt,
		UpdatedAt: fullOrder.UpdatedAt,
	}

	for _, item := range fullOrder.OrderItems {
		orderResponse.OrderItems = append(orderResponse.OrderItems, OrderItemResponse{
			OrderQuantity: item.Quantity,
			ID:            item.Product.ID,
			Name:          item.Product.Name,
			Rating:        item.Product.Rating,
			Quantity:      item.Product.Quantity,
			Description:   item.Product.Description,
			Images:        item.Product.Images,
			Price:         item.Product.Price,
			Info:          item.Product.Info,
			Feature:       item.Product.Feature,
			Guarantee:     item.Product.Guarantee,
			Discount:      item.Product.Discount,
			CreatedAt:     item.Product.CreatedAt,
			UpdatedAt:     item.Product.UpdatedAt,
			CategoryID:    item.Product.CategoryID,
			BrandID:       item.Product.BrandID,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(orderResponse)
}

func createLegalOrder(c *fiber.Ctx) error {
	type LegalOrderRequest struct {
		Price        float64 `json:"price" validate:"required,gte=0"`
		Bonus        float64 `json:"bonus" validate:"gte=0"`
		UserID       uint    `json:"user_id"`
		Status       string  `json:"status" validate:"required"`
		Service      string  `json:"service_mode" validate:"required"`
		Organization string  `json:"organization" validate:"required"`
		INN          string  `json:"inn" validate:"required"`
		Comment      string  `json:"comment"`
		OrderItems   []struct {
			ProductID uint `json:"product_id" validate:"required"`
			Quantity  int  `json:"quantity" validate:"required,gte=1"`
		} `json:"order_items" validate:"required,dive"`
	}

	var requestData LegalOrderRequest
	if err := c.BodyParser(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body: " + err.Error(),
		})
	}

	if err := validate.Struct(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// var user models.User
	// if err := db.DB.First(&user, requestData.UserID).Error; err != nil {
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"error": "User not found",
	// 	})
	// }

	order := models.Order{
		Price:        requestData.Price,
		Bonus:        requestData.Bonus,
		UserID:       requestData.UserID,
		Status:       requestData.Status,
		Service:      requestData.Service,
		OrderType:    "legal",
		Organization: requestData.Organization,
		INN:          requestData.INN,
		Comment:      requestData.Comment,
	}

	tx := db.DB.Begin()
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create legal order: " + err.Error(),
		})
	}

	var orderItems []models.OrderItem
	var calculatedPrice float64
	for _, item := range requestData.OrderItems {
		var product models.Product
		if err := tx.First(&product, item.ProductID).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Product %d not found", item.ProductID),
			})
		}

		if uint(item.Quantity) > product.Quantity {
			tx.Rollback()
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Insufficient quantity for product %d", item.ProductID),
			})
		}

		orderItems = append(orderItems, models.OrderItem{
			OrderID:   order.ID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
		calculatedPrice += product.Price * float64(item.Quantity)
	}

	// if calculatedPrice != requestData.Price {
	// 	tx.Rollback()
	// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"error":    "Price doesn't match order items total",
	// 		"expected": calculatedPrice,
	// 		"received": requestData.Price,
	// 	})
	// }

	if err := tx.Create(&orderItems).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create order items: " + err.Error(),
		})
	}

	for _, item := range orderItems {
		if err := tx.Model(&models.Product{}).
			Where("id = ?", item.ProductID).
			Update("quantity", gorm.Expr("quantity - ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update product quantities",
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction",
		})
	}

	// Verify the association (optional debug step)
	var checkUser models.User
	if err := db.DB.Preload("Orders").First(&checkUser, requestData.UserID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Order created but failed to verify user association",
		})
	}
	fmt.Printf("User orders after creation: %+v\n", checkUser.Orders) // Debug log

	var fullOrder models.Order
	if err := db.DB.Preload("OrderItems.Product").First(&fullOrder, order.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Order created but failed to load full details",
		})
	}

	orderResponse := OrderResponse{
		ID:           fullOrder.ID,
		Price:        fullOrder.Price,
		Bonus:        fullOrder.Bonus,
		UserID:       fullOrder.UserID,
		Status:       fullOrder.Status,
		Service:      fullOrder.Service,
		OrderType:    fullOrder.OrderType,
		Organization: fullOrder.Organization,
		INN:          fullOrder.INN,
		Comment:      fullOrder.Comment,
		CreatedAt:    fullOrder.CreatedAt,
		UpdatedAt:    fullOrder.UpdatedAt,
	}

	for _, item := range fullOrder.OrderItems {
		orderResponse.OrderItems = append(orderResponse.OrderItems, OrderItemResponse{
			OrderQuantity: item.Quantity,
			ID:            item.Product.ID,
			Name:          item.Product.Name,
			Rating:        item.Product.Rating,
			Quantity:      item.Product.Quantity,
			Description:   item.Product.Description,
			Images:        item.Product.Images,
			Price:         item.Product.Price,
			Info:          item.Product.Info,
			Feature:       item.Product.Feature,
			Guarantee:     item.Product.Guarantee,
			Discount:      item.Product.Discount,
			CreatedAt:     item.Product.CreatedAt,
			UpdatedAt:     item.Product.UpdatedAt,
			CategoryID:    item.Product.CategoryID,
			BrandID:       item.Product.BrandID,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(orderResponse)
}

func updateOrder(c *fiber.Ctx) error {
	// Request struct that combines both individual and legal order fields
	type UpdateOrderRequest struct {
		ID           uint    `json:"id" validate:"required"`
		Price        float64 `json:"price" validate:"gte=0"`
		Bonus        float64 `json:"bonus" validate:"gte=0"`
		Status       string  `json:"status"`
		Phone        string  `json:"phone"`        // For individual orders
		Name         string  `json:"name"`         // For individual orders
		Organization string  `json:"organization"` // For legal orders
		INN          string  `json:"inn"`          // For legal orders
		Comment      string  `json:"comment"`      // For legal orders
	}

	var requestData UpdateOrderRequest
	if err := c.BodyParser(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse request body: " + err.Error(),
		})
	}

	if err := validate.Struct(&requestData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"details": err.Error(),
		})
	}

	// Start transaction
	tx := db.DB.Begin()
	var order models.Order
	if err := tx.Preload("OrderItems").First(&order, requestData.ID).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found",
		})
	}

	// Update fields based on order type
	if order.OrderType == "individual" {
		if requestData.Phone != "" {
			order.Phone = requestData.Phone
		}
		if requestData.Name != "" {
			order.Name = requestData.Name
		}
	} else if order.OrderType == "legal" {
		if requestData.Organization != "" {
			order.Organization = requestData.Organization
		}
		if requestData.INN != "" {
			order.INN = requestData.INN
		}
		if requestData.Comment != "" {
			order.Comment = requestData.Comment
		}
	}

	// Update common fields if provided
	if requestData.Price > 0 {
		order.Price = requestData.Price
	}
	if requestData.Bonus > 0 {
		order.Bonus = requestData.Bonus
	}
	if requestData.Status != "" {
		order.Status = requestData.Status
	}

	// Save the updated order
	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update order: " + err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to commit transaction",
		})
	}

	// Load full order details for response
	var fullOrder models.Order
	if err := db.DB.Preload("OrderItems.Product").First(&fullOrder, order.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Order updated but failed to load full details",
		})
	}

	// Prepare response
	orderResponse := OrderResponse{
		ID:           fullOrder.ID,
		Price:        fullOrder.Price,
		Bonus:        fullOrder.Bonus,
		UserID:       fullOrder.UserID,
		Status:       fullOrder.Status,
		OrderType:    fullOrder.OrderType,
		Phone:        fullOrder.Phone,
		Name:         fullOrder.Name,
		Organization: fullOrder.Organization,
		INN:          fullOrder.INN,
		Comment:      fullOrder.Comment,
		CreatedAt:    fullOrder.CreatedAt,
		UpdatedAt:    fullOrder.UpdatedAt,
	}

	for _, item := range fullOrder.OrderItems {
		orderResponse.OrderItems = append(orderResponse.OrderItems, OrderItemResponse{
			OrderQuantity: item.Quantity,
			ID:            item.Product.ID,
			Name:          item.Product.Name,
			Rating:        item.Product.Rating,
			Quantity:      item.Product.Quantity,
			Description:   item.Product.Description,
			Images:        item.Product.Images,
			Price:         item.Product.Price,
			Info:          item.Product.Info,
			Feature:       item.Product.Feature,
			Guarantee:     item.Product.Guarantee,
			Discount:      item.Product.Discount,
			CreatedAt:     item.Product.CreatedAt,
			UpdatedAt:     item.Product.UpdatedAt,
			CategoryID:    item.Product.CategoryID,
			BrandID:       item.Product.BrandID,
		})
	}

	return c.Status(fiber.StatusOK).JSON(orderResponse)
}

func getAllOrders(c *fiber.Ctx) error {
	var orders []models.Order

	// Fetch orders with preloaded OrderItems and Products
	if err := db.DB.Preload("OrderItems.Product").Find(&orders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get orders",
		})
	}

	// Transform into response format
	var orderResponses []OrderResponse
	for _, order := range orders {
		orderResponse := OrderResponse{
			ID:           order.ID,
			Price:        order.Price,
			Bonus:        order.Bonus,
			UserID:       order.UserID,
			Status:       order.Status,
			Service:      order.Service,
			OrderType:    order.OrderType,
			Phone:        order.Phone,
			Name:         order.Name,
			Organization: order.Organization,
			INN:          order.INN,
			Comment:      order.Comment,
			CreatedAt:    order.CreatedAt,
			UpdatedAt:    order.UpdatedAt,
		}

		for _, item := range order.OrderItems {
			orderResponse.OrderItems = append(orderResponse.OrderItems, OrderItemResponse{
				OrderQuantity: item.Quantity,
				ID:            item.Product.ID,
				Name:          item.Product.Name,
				Rating:        item.Product.Rating,
				Quantity:      item.Product.Quantity,
				Description:   item.Product.Description,
				Images:        item.Product.Images,
				Price:         item.Product.Price,
				Info:          item.Product.Info,
				Feature:       item.Product.Feature,
				Guarantee:     item.Product.Guarantee,
				Discount:      item.Product.Discount,
				CreatedAt:     item.Product.CreatedAt,
				UpdatedAt:     item.Product.UpdatedAt,
				CategoryID:    item.Product.CategoryID,
				BrandID:       item.Product.BrandID,
			})
		}
		orderResponses = append(orderResponses, orderResponse)
	}

	return c.JSON(orderResponses)
}

func getOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	var order models.Order

	// Fetch order with preloaded OrderItems and Products
	if err := db.DB.Preload("OrderItems.Product").First(&order, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found",
		})
	}

	// Transform into response format
	orderResponse := OrderResponse{
		ID:           order.ID,
		Price:        order.Price,
		Bonus:        order.Bonus,
		UserID:       order.UserID,
		Status:       order.Status,
		Service:      order.Service,
		OrderType:    order.OrderType,
		Phone:        order.Phone,
		Name:         order.Name,
		Organization: order.Organization,
		INN:          order.INN,
		Comment:      order.Comment,
		CreatedAt:    order.CreatedAt,
		UpdatedAt:    order.UpdatedAt,
	}

	for _, item := range order.OrderItems {
		orderResponse.OrderItems = append(orderResponse.OrderItems, OrderItemResponse{
			OrderQuantity: item.Quantity,
			ID:            item.Product.ID,
			Name:          item.Product.Name,
			Rating:        item.Product.Rating,
			Quantity:      item.Product.Quantity,
			Description:   item.Product.Description,
			Images:        item.Product.Images,
			Price:         item.Product.Price,
			Info:          item.Product.Info,
			Feature:       item.Product.Feature,
			Guarantee:     item.Product.Guarantee,
			Discount:      item.Product.Discount,
			CreatedAt:     item.Product.CreatedAt,
			UpdatedAt:     item.Product.UpdatedAt,
			CategoryID:    item.Product.CategoryID,
			BrandID:       item.Product.BrandID,
		})
	}

	return c.JSON(orderResponse)
}

func deleteOrder(c *fiber.Ctx) error {
	id := c.Params("id")

	// Check if the order exists first
	var order models.Order
	if err := db.DB.First(&order, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found",
		})
	}

	// Delete the order
	if err := db.DB.Delete(&order).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete order",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Order deleted successfully",
	})
}

// Individual Order handlers
// func createIndividualOrder(c *fiber.Ctx) error {
// 	type IndividualOrderRequest struct {
// 		Price      float64 `json:"price"`
// 		Bonus      float64 `json:"bonus"`
// 		UserID     uint    `json:"user_id"`
// 		Phone      string  `json:"phone"`
// 		Name       string  `json:"name"`
// 		OrderItems []struct {
// 			ProductID uint `json:"product_id"`
// 			Quantity  int  `json:"quantity"`
// 		} `json:"order_items"`
// 	}

// 	var requestData IndividualOrderRequest
// 	if err := c.BodyParser(&requestData); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Failed to parse request body: " + err.Error(),
// 		})
// 	}

// 	order := models.Order{
// 		Price:     requestData.Price,
// 		Bonus:     requestData.Bonus,
// 		UserID:    requestData.UserID,
// 		OrderType: "individual",
// 		Phone:     requestData.Phone,
// 		Name:      requestData.Name,
// 	}

// 	if err := validate.Struct(&order); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error":   "Validation failed",
// 			"details": err.Error(),
// 		})
// 	}

// 	tx := db.DB.Begin()

// 	if err := tx.Create(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create individual order: " + err.Error(),
// 		})
// 	}

// 	var orderItems []models.OrderItem
// 	for _, item := range requestData.OrderItems {
// 		var product models.Product
// 		if err := tx.First(&product, item.ProductID).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Product not found: " + err.Error(),
// 			})
// 		}

// 		orderItems = append(orderItems, models.OrderItem{
// 			OrderID:   order.ID,
// 			ProductID: item.ProductID,
// 			Quantity:  item.Quantity,
// 		})
// 	}

// 	if len(orderItems) > 0 {
// 		if err := tx.Create(&orderItems).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create order items: " + err.Error(),
// 			})
// 		}
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	var fullOrder models.Order
// 	if err := db.DB.Preload("OrderItems.Product").First(&fullOrder, order.ID).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Order created but failed to load full details",
// 		})
// 	}

// 	return c.Status(fiber.StatusCreated).JSON(fullOrder)
// }

// func getAllIndividualOrders(c *fiber.Ctx) error {
// 	var orders []models.IndividualOrder

// 	if err := db.DB.Where("order_type = ?", "individual").
// 		Preload("OrderItems.Product").
// 		Find(&orders).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to get individual orders",
// 		})
// 	}

// 	return c.JSON(orders)
// }

// func getIndividualOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")
// 	var order models.IndividualOrder

// 	if err := db.DB.Where("id = ? AND order_type = ?", id, "individual").
// 		Preload("OrderItems.Product").
// 		First(&order).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Individual order not found",
// 		})
// 	}

// 	return c.JSON(order)
// }

// func updateIndividualOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")
// 	updateData := new(models.IndividualOrder)

// 	if err := c.BodyParser(updateData); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Failed to parse request body",
// 		})
// 	}

// 	// Parse product items separately (if provided)
// 	var orderRequest models.OrderRequest
// 	hasProducts := false
// 	if err := c.BodyParser(&orderRequest); err == nil && len(orderRequest.Items) > 0 {
// 		hasProducts = true
// 	}

// 	// Get existing order
// 	var existingOrder models.IndividualOrder
// 	if err := db.DB.Where("id = ? AND order_type = ?", id, "individual").First(&existingOrder).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Individual order not found",
// 		})
// 	}

// 	// Begin transaction
// 	tx := db.DB.Begin()

// 	// Update order basic details
// 	updateMap := map[string]interface{}{
// 		"phone": updateData.Phone,
// 		"name":  updateData.Name,
// 		"price": updateData.Price,
// 		"bonus": updateData.Bonus,
// 	}

// 	if err := tx.Model(&models.IndividualOrder{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update individual order",
// 		})
// 	}

// 	// Update order items if provided
// 	if hasProducts {
// 		// Delete existing order items
// 		if err := tx.Where("order_id = ?", id).Delete(&models.OrderItem{}).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to update order items",
// 			})
// 		}

// 		// Create new order items
// 		var orderItems []models.OrderItem
// 		for _, item := range orderRequest.Items {
// 			// Verify product exists
// 			var product models.Product
// 			if err := tx.First(&product, item.ProductID).Error; err != nil {
// 				tx.Rollback()
// 				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 					"error": "Product not found: " + err.Error(),
// 				})
// 			}

// 			orderItem := models.OrderItem{
// 				OrderID:   existingOrder.ID,
// 				ProductID: item.ProductID,
// 				Quantity:  item.Quantity,
// 			}
// 			orderItems = append(orderItems, orderItem)
// 		}

// 		// Save new order items
// 		if len(orderItems) > 0 {
// 			if err := tx.Create(&orderItems).Error; err != nil {
// 				tx.Rollback()
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": "Failed to create order items: " + err.Error(),
// 				})
// 			}
// 		}
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"success": true,
// 		"message": "Individual order updated successfully",
// 	})
// }

// func deleteIndividualOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")

// 	// Begin transaction
// 	tx := db.DB.Begin()

// 	// Delete associated order items first
// 	if err := tx.Where("order_id = ?", id).Delete(&models.OrderItem{}).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to delete order items",
// 		})
// 	}

// 	// Check if the order exists
// 	var order models.IndividualOrder
// 	if err := tx.Where("id = ? AND order_type = ?", id, "individual").First(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Individual order not found",
// 		})
// 	}

// 	// Delete the order
// 	if err := tx.Delete(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to delete individual order",
// 		})
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"success": true,
// 		"message": "Individual order deleted successfully",
// 	})
// }

// Legal Order handlers
// func createLegalOrder(c *fiber.Ctx) error {
// 	// Create a temporary struct to parse the request
// 	type LegalOrderRequest struct {
// 		Price        float64 `json:"price"`
// 		Bonus        float64 `json:"bonus"`
// 		UserID       uint    `json:"user_id"`
// 		Organization string  `json:"organization"`
// 		INN          string  `json:"inn"`
// 		Comment      string  `json:"comment"`
// 		OrderItems   []struct {
// 			ProductID uint `json:"product_id"`
// 			Quantity  int  `json:"quantity"`
// 		} `json:"order_items"`
// 	}

// 	// Parse the request
// 	var requestData LegalOrderRequest
// 	if err := c.BodyParser(&requestData); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Failed to parse request body: " + err.Error(),
// 		})
// 	}

// 	// Create the order
// 	order := models.Order{
// 		Price:        requestData.Price,
// 		Bonus:        requestData.Bonus,
// 		UserID:       requestData.UserID,
// 		OrderType:    "legal",
// 		Organization: requestData.Organization,
// 		INN:          requestData.INN,
// 		Comment:      requestData.Comment,
// 	}

// 	// Validate the order
// 	if err := validate.Struct(&order); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error":   "Validation failed",
// 			"details": err.Error(),
// 		})
// 	}

// 	// Begin transaction
// 	tx := db.DB.Begin()

// 	// Save the order
// 	if err := tx.Create(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to create legal order: " + err.Error(),
// 		})
// 	}

// 	// Process order items
// 	var orderItems []models.OrderItem
// 	for _, item := range requestData.OrderItems {
// 		// Verify product exists
// 		var product models.Product
// 		if err := tx.First(&product, item.ProductID).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Product not found: " + err.Error(),
// 			})
// 		}

// 		orderItems = append(orderItems, models.OrderItem{
// 			OrderID:   order.ID,
// 			ProductID: item.ProductID,
// 			Quantity:  item.Quantity,
// 		})
// 	}

// 	// Save order items
// 	if len(orderItems) > 0 {
// 		if err := tx.Create(&orderItems).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to create order items: " + err.Error(),
// 			})
// 		}
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	// Load the full order with items for the response
// 	var fullOrder models.Order
// 	if err := db.DB.Preload("OrderItems.Product").First(&fullOrder, order.ID).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Order created but failed to load full details",
// 		})
// 	}

// 	return c.Status(fiber.StatusCreated).JSON(fullOrder)
// }

// func getAllLegalOrders(c *fiber.Ctx) error {
// 	var orders []models.LegalOrder

// 	if err := db.DB.Where("order_type = ?", "legal").
// 		Preload("OrderItems.Product").
// 		Find(&orders).Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to get legal orders",
// 		})
// 	}

// 	return c.JSON(orders)
// }

// func getLegalOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")
// 	var order models.LegalOrder

// 	if err := db.DB.Where("id = ? AND order_type = ?", id, "legal").
// 		Preload("OrderItems.Product").
// 		First(&order).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Legal order not found",
// 		})
// 	}

// 	return c.JSON(order)
// }

// func updateLegalOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")
// 	updateData := new(models.LegalOrder)

// 	if err := c.BodyParser(updateData); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 			"error": "Failed to parse request body",
// 		})
// 	}

// 	// Parse product items separately (if provided)
// 	var orderRequest models.OrderRequest
// 	hasProducts := false
// 	if err := c.BodyParser(&orderRequest); err == nil && len(orderRequest.Items) > 0 {
// 		hasProducts = true
// 	}

// 	// Get existing order
// 	var existingOrder models.LegalOrder
// 	if err := db.DB.Where("id = ? AND order_type = ?", id, "legal").First(&existingOrder).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Legal order not found",
// 		})
// 	}

// 	// Begin transaction
// 	tx := db.DB.Begin()

// 	// Update order basic details
// 	updateMap := map[string]interface{}{
// 		"organization": updateData.Organization,
// 		"inn":          updateData.INN,
// 		"comment":      updateData.Comment,
// 		"price":        updateData.Price,
// 		"bonus":        updateData.Bonus,
// 	}

// 	if err := tx.Model(&models.LegalOrder{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to update legal order",
// 		})
// 	}

// 	// Update order items if provided
// 	if hasProducts {
// 		// Delete existing order items
// 		if err := tx.Where("order_id = ?", id).Delete(&models.OrderItem{}).Error; err != nil {
// 			tx.Rollback()
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": "Failed to update order items",
// 			})
// 		}

// 		// Create new order items
// 		var orderItems []models.OrderItem
// 		for _, item := range orderRequest.Items {
// 			// Verify product exists
// 			var product models.Product
// 			if err := tx.First(&product, item.ProductID).Error; err != nil {
// 				tx.Rollback()
// 				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 					"error": "Product not found: " + err.Error(),
// 				})
// 			}

// 			orderItem := models.OrderItem{
// 				OrderID:   existingOrder.ID,
// 				ProductID: item.ProductID,
// 				Quantity:  item.Quantity,
// 			}
// 			orderItems = append(orderItems, orderItem)
// 		}

// 		// Save new order items
// 		if len(orderItems) > 0 {
// 			if err := tx.Create(&orderItems).Error; err != nil {
// 				tx.Rollback()
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": "Failed to create order items: " + err.Error(),
// 				})
// 			}
// 		}
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"success": true,
// 		"message": "Legal order updated successfully",
// 	})
// }

// func deleteLegalOrder(c *fiber.Ctx) error {
// 	id := c.Params("id")

// 	// Begin transaction
// 	tx := db.DB.Begin()

// 	// Delete associated order items first
// 	if err := tx.Where("order_id = ?", id).Delete(&models.OrderItem{}).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to delete order items",
// 		})
// 	}

// 	// Check if the order exists
// 	var order models.LegalOrder
// 	if err := tx.Where("id = ? AND order_type = ?", id, "legal").First(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Legal order not found",
// 		})
// 	}

// 	// Delete the order
// 	if err := tx.Delete(&order).Error; err != nil {
// 		tx.Rollback()
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to delete legal order",
// 		})
// 	}

// 	// Commit transaction
// 	if err := tx.Commit().Error; err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 			"error": "Failed to commit transaction",
// 		})
// 	}

// 	return c.JSON(fiber.Map{
// 		"success": true,
// 		"message": "Legal order deleted successfully",
// 	})
// }

// Generic Order handlers
