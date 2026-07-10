package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/shelly-app/shelly/internal/api"
	"github.com/shelly-app/shelly/internal/config"
	"github.com/shelly-app/shelly/internal/database"
	"github.com/shelly-app/shelly/internal/middleware"
)

func main() {
	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init database
	if err := database.Init(&cfg.Database); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// Init crypto
	if err := api.InitCrypto(cfg.Crypto.Key); err != nil {
		log.Fatalf("Failed to init crypto: %v", err)
	}

	// Setup Gin
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(middleware.CORS())

	// Setup API routes
	api.SetupRouter(r)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Serve embedded frontend (must be last)
	ServeEmbeddedFrontend(r)

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Shelly server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
