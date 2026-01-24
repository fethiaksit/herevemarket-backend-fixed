package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"backend/internal/config"
	"backend/internal/database"
	"backend/internal/handlers"
	"backend/internal/middleware"
)

func main() {
	config.Load()

	client, err := database.Connect(config.AppEnv.MongoURI)
	if err != nil {
		log.Fatal(err)
	}

	db := client.Database(config.AppEnv.DBName)

	log.Println("MongoDB connected to:", db.Name())

	if err := database.EnsureProductIndexes(db); err != nil {
		log.Println("⚠️ product index warning: %v", err)
	}
	if err := database.EnsureUserIndexes(db); err != nil {
		log.Println("⚠️ user index warning: %v", err)
	}
	if err := database.EnsureOrderIndexes(db); err != nil {
		log.Println("⚠️ order index warning: %v", err)
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/**/*")
	r.Static("/public", "./public")

	r.GET("/", handlers.Home())
	r.GET("/admin/login", handlers.AdminLoginPage)
	r.GET("/admin/categories", handlers.AdminCategoriesPage)
	r.GET("/admin/products", handlers.AdminProductsPage)
	r.GET("/admin/orders", handlers.AdminOrdersPage)

	r.POST("/auth/register", handlers.Register(db, config.AppEnv.JWTSecret, config.AppEnv.AccessTokenTTL))
	r.POST("/auth/login", handlers.Login(
		db,
		config.AppEnv.JWTSecret,
		config.AppEnv.AccessTokenTTL,
		config.AppEnv.RefreshTokenTTL,
	))
	r.GET("/auth/me", middleware.UserAuth(config.AppEnv.JWTSecret), handlers.GetMe(db))
	r.POST("/auth/refresh", handlers.Refresh(
		db,
		config.AppEnv.JWTSecret,
		config.AppEnv.AccessTokenTTL,
		config.AppEnv.RefreshTokenTTL,
	))
	r.POST("/auth/logout", handlers.Logout(db))

	r.POST("/admin/login", handlers.AdminLogin(db, config.AppEnv.JWTSecret, config.AppEnv.AccessTokenTTL))

	r.GET("/products", handlers.GetProducts(db))
	r.GET("/categories", handlers.GetCategories(db))
	r.GET("/products/campaign", handlers.GetCampaignProducts(db))
	r.POST("/orders", handlers.CreateOrder(db, config.AppEnv.JWTSecret))
	r.GET("/orders", handlers.GetOrders(db))

	user := r.Group("/user")
	user.Use(middleware.UserAuth(config.AppEnv.JWTSecret))
	{
		user.GET("/addresses", handlers.GetUserAddresses(db))
		user.POST("/addresses", handlers.CreateUserAddress(db))
		user.PUT("/addresses/:id", handlers.UpdateUserAddress(db))
		user.DELETE("/addresses/:id", handlers.DeleteUserAddress(db))
	}

	admin := r.Group("/admin/api")
	admin.Use(middleware.AdminAuth(config.AppEnv.JWTSecret))
	{
		admin.GET("/me", func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

		admin.GET("/products", handlers.GetAllProducts(db))
		admin.POST("/products", handlers.CreateProduct(db))
		admin.PUT("/products/:id", handlers.UpdateProduct(db))
		admin.DELETE("/products/:id", handlers.DeleteProduct(db))

		admin.GET("/categories", handlers.GetAllCategories(db))
		admin.POST("/categories", handlers.CreateCategory(db))
		admin.PUT("/categories/:id", handlers.UpdateCategory(db))
		admin.DELETE("/categories/:id", handlers.DeleteCategory(db))

		admin.DELETE("/orders/:id", handlers.DeleteOrder(db))
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
