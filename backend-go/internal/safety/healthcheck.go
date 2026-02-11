package safety

import (
	"context"
	"log"
	"sync"
	"time"
)

// HealthProbe is the interface that health check probes must implement
type HealthProbe interface {
	Execute(ctx context.Context) (passed bool, err error)
	Name() string
}

// HealthCheckLoop monitors probes during experiments and triggers rollback
// when consecutive failures exceed the threshold
type HealthCheckLoop struct {
	experimentID     string
	probes           []HealthProbe
	interval         time.Duration
	failureThreshold int
	onFailure        func()
	rollbackMgr      *RollbackManager

	mu                  sync.Mutex
	consecutiveFailures int
	running             bool
	cancel              context.CancelFunc
}

// NewHealthCheckLoop creates a new health check loop
func NewHealthCheckLoop(
	experimentID string,
	probes []HealthProbe,
	interval time.Duration,
	failureThreshold int,
	rollbackMgr *RollbackManager,
) *HealthCheckLoop {
	return &HealthCheckLoop{
		experimentID:     experimentID,
		probes:           probes,
		interval:         interval,
		failureThreshold: failureThreshold,
		rollbackMgr:      rollbackMgr,
	}
}

// Start begins the health check polling loop in a goroutine
func (hc *HealthCheckLoop) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true

	ctx, cancel := context.WithCancel(context.Background())
	hc.cancel = cancel
	hc.mu.Unlock()

	log.Printf("Health check loop started for %s (interval=%v, threshold=%d)",
		hc.experimentID, hc.interval, hc.failureThreshold)

	go hc.run(ctx)
}

// Stop halts the health check loop
func (hc *HealthCheckLoop) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}
	hc.running = false
	if hc.cancel != nil {
		hc.cancel()
	}
	log.Printf("Health check loop stopped for %s", hc.experimentID)
}

// IsRunning returns whether the loop is currently active
func (hc *HealthCheckLoop) IsRunning() bool {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.running
}

func (hc *HealthCheckLoop) run(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			allPassed := hc.checkProbes(ctx)
			if allPassed {
				hc.consecutiveFailures = 0
				continue
			}

			hc.consecutiveFailures++
			log.Printf("Health check failed for %s (%d/%d)",
				hc.experimentID, hc.consecutiveFailures, hc.failureThreshold)

			if hc.consecutiveFailures >= hc.failureThreshold {
				log.Printf("Health check threshold reached for %s. Triggering rollback.",
					hc.experimentID)

				if hc.onFailure != nil {
					hc.onFailure()
				} else if hc.rollbackMgr != nil {
					hc.rollbackMgr.Rollback(hc.experimentID)
				}

				hc.Stop()
				return
			}
		}
	}
}

func (hc *HealthCheckLoop) checkProbes(ctx context.Context) bool {
	if len(hc.probes) == 0 {
		return true
	}

	for _, probe := range hc.probes {
		passed, err := probe.Execute(ctx)
		if err != nil || !passed {
			return false
		}
	}
	return true
}
