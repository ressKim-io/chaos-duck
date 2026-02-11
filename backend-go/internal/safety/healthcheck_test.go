package safety

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockProbe implements HealthProbe for testing
type mockProbe struct {
	name   string
	passed bool
	err    error
}

func (p *mockProbe) Execute(ctx context.Context) (bool, error) {
	return p.passed, p.err
}

func (p *mockProbe) Name() string { return p.name }

func TestHealthCheckLoopStartStop(t *testing.T) {
	rm := NewRollbackManager()
	probe := &mockProbe{name: "test", passed: true}

	hc := NewHealthCheckLoop("exp-1", []HealthProbe{probe}, 100*time.Millisecond, 3, rm)

	assert.False(t, hc.IsRunning())

	hc.Start()
	assert.True(t, hc.IsRunning())

	// Starting again should be a no-op
	hc.Start()
	assert.True(t, hc.IsRunning())

	hc.Stop()
	time.Sleep(50 * time.Millisecond)
	assert.False(t, hc.IsRunning())

	// Stopping again should be a no-op
	hc.Stop()
}

func TestHealthCheckLoopFailureThreshold(t *testing.T) {
	rm := NewRollbackManager()
	rm.Push("exp-1", func() (map[string]any, error) {
		return map[string]any{"rolled_back": true}, nil
	}, "test-action")

	// Probe always fails
	probe := &mockProbe{name: "failing", passed: false}

	hc := NewHealthCheckLoop("exp-1", []HealthProbe{probe}, 50*time.Millisecond, 2, rm)
	hc.Start()

	// Wait for failure threshold to be reached
	time.Sleep(300 * time.Millisecond)

	assert.False(t, hc.IsRunning(), "loop should stop after reaching threshold")
	assert.Equal(t, 0, rm.StackSize("exp-1"), "rollback should have been triggered")
}

func TestHealthCheckLoopAllPassing(t *testing.T) {
	rm := NewRollbackManager()
	rm.Push("exp-1", func() (map[string]any, error) {
		return nil, nil
	}, "should-not-rollback")

	probe := &mockProbe{name: "healthy", passed: true}

	hc := NewHealthCheckLoop("exp-1", []HealthProbe{probe}, 50*time.Millisecond, 3, rm)
	hc.Start()

	time.Sleep(200 * time.Millisecond)
	hc.Stop()

	// Rollback should NOT have been triggered
	assert.Equal(t, 1, rm.StackSize("exp-1"))
}

func TestHealthCheckLoopNoProbes(t *testing.T) {
	rm := NewRollbackManager()

	hc := NewHealthCheckLoop("exp-1", []HealthProbe{}, 50*time.Millisecond, 3, rm)
	hc.Start()

	time.Sleep(150 * time.Millisecond)
	hc.Stop()

	// Should not crash or trigger rollback with empty probes
	assert.False(t, hc.IsRunning())
}

func TestHealthCheckLoopOnFailureCallback(t *testing.T) {
	rm := NewRollbackManager()
	probe := &mockProbe{name: "failing", passed: false}

	var callbackCalled atomic.Bool

	hc := NewHealthCheckLoop("exp-1", []HealthProbe{probe}, 50*time.Millisecond, 1, rm)
	hc.onFailure = func() {
		callbackCalled.Store(true)
	}
	hc.Start()

	time.Sleep(200 * time.Millisecond)

	assert.True(t, callbackCalled.Load(), "on_failure callback should have been called")
}
