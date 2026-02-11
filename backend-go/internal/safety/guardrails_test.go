package safety

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestEmergencyStopManager(t *testing.T) {
	esm := NewEmergencyStopManager()

	assert.False(t, esm.IsTriggered())
	assert.NoError(t, esm.CheckEmergencyStop())

	esm.Trigger()
	assert.True(t, esm.IsTriggered())
	assert.ErrorIs(t, esm.CheckEmergencyStop(), domain.ErrEmergencyStop)

	esm.Reset()
	assert.False(t, esm.IsTriggered())
	assert.NoError(t, esm.CheckEmergencyStop())
}

func TestEmergencyStopConcurrency(t *testing.T) {
	esm := NewEmergencyStopManager()

	// Trigger and reset from multiple goroutines
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			esm.Trigger()
			_ = esm.IsTriggered()
			esm.Reset()
			_ = esm.CheckEmergencyStop()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestWithTimeoutSuccess(t *testing.T) {
	err := WithTimeout(context.Background(), 5, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestWithTimeoutExpired(t *testing.T) {
	err := WithTimeout(context.Background(), 1, func(ctx context.Context) error {
		time.Sleep(3 * time.Second)
		return nil
	})
	assert.ErrorIs(t, err, domain.ErrTimeout)
}

func TestWithTimeoutFunctionError(t *testing.T) {
	expectedErr := errors.New("something went wrong")
	err := WithTimeout(context.Background(), 5, func(ctx context.Context) error {
		return expectedErr
	})
	assert.ErrorIs(t, err, expectedErr)
}

func TestWithTimeoutClamp(t *testing.T) {
	// Values below 1 should be clamped to 1
	err := WithTimeout(context.Background(), 0, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Values above 120 should be clamped to 120 (we just verify it doesn't panic)
	err = WithTimeout(context.Background(), 999, func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestValidateBlastRadius(t *testing.T) {
	tests := []struct {
		name     string
		affected int
		total    int
		maxRatio float64
		wantErr  bool
	}{
		{"within limit", 1, 10, 0.3, false},
		{"at limit", 3, 10, 0.3, false},
		{"exceeds limit", 4, 10, 0.3, true},
		{"zero total", 0, 0, 0.3, false},
		{"all affected", 10, 10, 0.3, true},
		{"100% allowed", 10, 10, 1.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlastRadius(tt.affected, tt.total, tt.maxRatio)
			if tt.wantErr {
				assert.ErrorIs(t, err, domain.ErrBlastRadiusExceeded)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRequireConfirmation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		pattern   string
		confirmed bool
		wantErr   bool
	}{
		{"prod without confirmation", "production", "prod*", false, true},
		{"prod with confirmation", "production", "prod*", true, false},
		{"non-prod namespace", "staging", "prod*", false, false},
		{"empty pattern defaults to prod*", "production", "", false, true},
		{"non-matching pattern", "staging", "prod*", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireConfirmation(tt.namespace, tt.pattern, tt.confirmed)
			if tt.wantErr {
				assert.ErrorIs(t, err, domain.ErrNamespaceConfirmation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
