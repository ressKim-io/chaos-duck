package safety

import (
	"context"
	"log"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
)

// EmergencyStopManager manages the global emergency stop flag
type EmergencyStopManager struct {
	triggered atomic.Bool
}

// NewEmergencyStopManager creates a new EmergencyStopManager
func NewEmergencyStopManager() *EmergencyStopManager {
	return &EmergencyStopManager{}
}

// Trigger activates the emergency stop
func (esm *EmergencyStopManager) Trigger() {
	log.Println("EMERGENCY STOP TRIGGERED")
	esm.triggered.Store(true)
}

// Reset clears the emergency stop, allowing new experiments
func (esm *EmergencyStopManager) Reset() {
	esm.triggered.Store(false)
	log.Println("Emergency stop reset")
}

// IsTriggered returns whether emergency stop is active
func (esm *EmergencyStopManager) IsTriggered() bool {
	return esm.triggered.Load()
}

// CheckEmergencyStop returns ErrEmergencyStop if triggered
func (esm *EmergencyStopManager) CheckEmergencyStop() error {
	if esm.triggered.Load() {
		return domain.ErrEmergencyStop
	}
	return nil
}

// WithTimeout wraps a function call with a context timeout.
// Max allowed timeout is 120 seconds; values are clamped.
func WithTimeout(ctx context.Context, seconds int, fn func(ctx context.Context) error) error {
	if seconds < 1 {
		seconds = 1
	}
	if seconds > 120 {
		seconds = 120
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return domain.ErrTimeout
	}
}

// ValidateBlastRadius checks that the affected ratio does not exceed the limit
func ValidateBlastRadius(affected, total int, maxRatio float64) error {
	if total == 0 {
		return nil
	}
	ratio := float64(affected) / float64(total)
	if ratio > maxRatio {
		log.Printf("Blast radius %.1f%% exceeds max %.1f%%", ratio*100, maxRatio*100)
		return domain.ErrBlastRadiusExceeded
	}
	return nil
}

// RequireConfirmation checks if a namespace matches a production pattern
// and ensures explicit confirmation is set
func RequireConfirmation(namespace, pattern string, confirmed bool) error {
	if pattern == "" {
		pattern = "prod*"
	}
	matched, _ := filepath.Match(pattern, namespace)
	if matched && !confirmed {
		return domain.ErrNamespaceConfirmation
	}
	return nil
}
