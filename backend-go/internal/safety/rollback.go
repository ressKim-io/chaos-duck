package safety

import (
	"log"
	"sync"

	"github.com/chaosduck/backend-go/internal/domain"
)

// rollbackEntry pairs a description with its undo function
type rollbackEntry struct {
	Description string
	Fn          domain.RollbackFunc
}

// RollbackResult describes the outcome of a single rollback operation
type RollbackResult struct {
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
}

// RollbackManager maintains per-experiment LIFO rollback stacks
type RollbackManager struct {
	mu     sync.Mutex
	stacks map[string][]rollbackEntry
}

// NewRollbackManager creates a new RollbackManager
func NewRollbackManager() *RollbackManager {
	return &RollbackManager{
		stacks: make(map[string][]rollbackEntry),
	}
}

// Push adds a rollback function to the experiment's stack
func (rm *RollbackManager) Push(experimentID string, fn domain.RollbackFunc, description string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.stacks[experimentID] = append(rm.stacks[experimentID], rollbackEntry{
		Description: description,
		Fn:          fn,
	})
	log.Printf("Rollback pushed for %s: %s (stack size: %d)",
		experimentID, description, len(rm.stacks[experimentID]))
}

// Rollback executes all rollback functions for an experiment in LIFO order
func (rm *RollbackManager) Rollback(experimentID string) []RollbackResult {
	rm.mu.Lock()
	stack := rm.stacks[experimentID]
	delete(rm.stacks, experimentID)
	rm.mu.Unlock()

	var results []RollbackResult

	// Execute in reverse (LIFO)
	for i := len(stack) - 1; i >= 0; i-- {
		entry := stack[i]
		result, err := entry.Fn()
		if err != nil {
			results = append(results, RollbackResult{
				Description: entry.Description,
				Status:      "failed",
				Error:       err.Error(),
			})
			log.Printf("Rollback failed: %s - %v", entry.Description, err)
		} else {
			results = append(results, RollbackResult{
				Description: entry.Description,
				Status:      "success",
				Result:      result,
			})
			log.Printf("Rollback success: %s", entry.Description)
		}
	}

	return results
}

// RollbackAll executes rollback for ALL active experiments (emergency stop)
func (rm *RollbackManager) RollbackAll() map[string][]RollbackResult {
	rm.mu.Lock()
	ids := make([]string, 0, len(rm.stacks))
	for id := range rm.stacks {
		ids = append(ids, id)
	}
	rm.mu.Unlock()

	all := make(map[string][]RollbackResult)
	for _, id := range ids {
		all[id] = rm.Rollback(id)
	}
	return all
}

// StackSize returns the number of rollback entries for an experiment
func (rm *RollbackManager) StackSize(experimentID string) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return len(rm.stacks[experimentID])
}

// ActiveExperiments returns IDs of experiments with pending rollbacks
func (rm *RollbackManager) ActiveExperiments() []string {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	ids := make([]string, 0, len(rm.stacks))
	for id := range rm.stacks {
		ids = append(ids, id)
	}
	return ids
}
