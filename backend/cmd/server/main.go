package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"forest-bd-viewer/internal/auth"
	"forest-bd-viewer/internal/cache"
	"forest-bd-viewer/internal/config"
	"forest-bd-viewer/internal/database"
	"forest-bd-viewer/internal/geo"
	"forest-bd-viewer/internal/graph"
	"forest-bd-viewer/internal/graph/generated"
	"forest-bd-viewer/internal/tiles"
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

	// Initialize auth service
	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTExpiryHours)

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

	// Inject Echo context into request context so GraphQL resolvers can set cookies
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := context.WithValue(c.Request().Context(), graph.EchoContextKey, c)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})

	// JWT auth middleware (must run after echo context injection)
	e.Use(authSvc.Middleware())

	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "connected",
			"redis":    "connected",
		})
	})

	// GraphQL endpoint
	graphqlHandler := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: &graph.Resolver{DB: pool, AuthSvc: authSvc},
	}))
	e.POST("/graphql", echo.WrapHandler(graphqlHandler))
	e.GET("/graphql", echo.WrapHandler(graphqlHandler))

	// MVT tile endpoints
	geoQueries := &geo.Queries{DB: pool}
	tileHandler := tiles.NewHandler(geoQueries, redisClient)
	e.GET("/tiles/foret/:z/:x/:y", tileHandler.ForestTile)
	e.GET("/tiles/admin/:layer/:z/:x/:y", tileHandler.AdminTile)
	e.GET("/tiles/cadastre/:z/:x/:y", tileHandler.CadastreTile)

	// LiDAR CHM image endpoint
	e.GET("/lidar/chm/:id", func(c echo.Context) error {
		imageID := c.Param("id")
		imageID = strings.TrimSuffix(imageID, ".png")
		path, err := geo.ServeCHMImage(imageID)
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "CHM image not found"})
		}
		return c.File(path)
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.BackendPort)
	fmt.Printf("Backend server starting on %s\n", addr)
	e.Logger.Fatal(e.Start(addr))
}
