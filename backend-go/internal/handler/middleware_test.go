package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "/api/chaos/experiments", "/api/chaos/experiments"},
		{"with short ID", "/api/chaos/experiments/a1b2c3d4", "/api/chaos/experiments/{id}"},
		{"with dry prefix", "/api/chaos/experiments/dry-a1b2c3d4", "/api/chaos/experiments/{id}"},
		{"root path", "/", "/"},
		{"health", "/health", "/health"},
		{"metrics", "/metrics", "/metrics"},
		{"topology", "/api/topology/k8s", "/api/topology/k8s"},
		{"trailing slash", "/api/chaos/experiments/", "/api/chaos/experiments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsShortID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"a1b2c3d4", true},
		{"12345678", true},
		{"abcdef01", true},
		{"abcd-f01", true},  // contains dash
		{"ABCDEF01", false}, // uppercase not matched
		{"abc", false},      // too short
		{"a1b2c3d4e5", false}, // too long
		{"zzzzzzzz", false}, // non-hex chars
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isShortID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
