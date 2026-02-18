package main

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"forest-bd-viewer/internal/cache"
	"forest-bd-viewer/internal/config"
	"forest-bd-viewer/internal/database"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	pool := database.NewPool(cfg.DatabaseURL())
	defer pool.Close()

	// Initialize Redis
	redisClient := cache.NewRedisClient(cfg.RedisAddr())
	defer redisClient.Close()

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:3000"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "connected",
			"redis":    "connected",
		})
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.BackendPort)
	fmt.Printf("Backend server starting on %s\n", addr)
	e.Logger.Fatal(e.Start(addr))
}
