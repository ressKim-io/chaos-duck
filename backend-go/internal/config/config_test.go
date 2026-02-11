package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Contains(t, cfg.DatabaseURL, "chaosduck")
	assert.Equal(t, "http://localhost:8001", cfg.AIServiceURL)
	assert.Equal(t, "us-east-1", cfg.AWSRegion)
	assert.Equal(t, "http://localhost:5173", cfg.CORSAllowOrigin)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("AI_SERVICE_URL", "http://ai:8001")
	t.Setenv("AWS_DEFAULT_REGION", "ap-northeast-2")

	cfg := Load()

	assert.Equal(t, "9090", cfg.ServerPort)
	assert.Equal(t, "http://ai:8001", cfg.AIServiceURL)
	assert.Equal(t, "ap-northeast-2", cfg.AWSRegion)
}

func TestEnvInt(t *testing.T) {
	assert.Equal(t, 42, EnvInt("NONEXISTENT_VAR", 42))

	t.Setenv("TEST_INT", "100")
	assert.Equal(t, 100, EnvInt("TEST_INT", 42))

	t.Setenv("TEST_BAD_INT", "notanumber")
	assert.Equal(t, 42, EnvInt("TEST_BAD_INT", 42))
}
