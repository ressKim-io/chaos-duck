package main

import (
	"log"
	"net/http"

	"github.com/chaosduck/backend-go/internal/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"emergency_stop": false,
		})
	})

	log.Printf("ChaosDuck backend-go starting on :%s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
