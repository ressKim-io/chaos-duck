package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server
	ServerPort string

	// Database
	DatabaseURL string

	// AI Service
	AIServiceURL string

	// AWS
	AWSRegion string

	// CORS
	CORSAllowOrigin string

	// Kubernetes
	KubeConfig string
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		ServerPort:      envOrDefault("SERVER_PORT", "8080"),
		DatabaseURL:     envOrDefault("DATABASE_URL", "postgres://chaosduck:chaosduck@localhost:5432/chaosduck?sslmode=disable"),
		AIServiceURL:    envOrDefault("AI_SERVICE_URL", "http://localhost:8001"),
		AWSRegion:       envOrDefault("AWS_DEFAULT_REGION", "us-east-1"),
		CORSAllowOrigin: envOrDefault("CORS_ALLOW_ORIGIN", "http://localhost:5173"),
		KubeConfig:      envOrDefault("KUBECONFIG", ""),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvInt reads an integer environment variable with a fallback
func EnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
