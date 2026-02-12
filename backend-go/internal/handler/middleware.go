package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/chaosduck/backend-go/internal/observability"
	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware records HTTP request metrics
func PrometheusMiddleware(metrics *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := normalizePath(c.Request.URL.Path)
		method := c.Request.Method

		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()

		status := fmt.Sprintf("%d", c.Writer.Status())
		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware(allowOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// normalizePath replaces dynamic path segments with placeholders
func normalizePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	normalized := make([]string, 0, len(parts))

	for _, part := range parts {
		if isShortID(part) || strings.HasPrefix(part, "dry-") {
			normalized = append(normalized, "{id}")
		} else {
			normalized = append(normalized, part)
		}
	}

	return "/" + strings.Join(normalized, "/")
}

// isShortID checks if a string looks like an 8-char hex ID
func isShortID(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && c != '-' {
			return false
		}
	}
	return true
}
