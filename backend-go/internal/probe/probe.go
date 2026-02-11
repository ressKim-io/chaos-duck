package probe

import (
	"context"
	"log"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
)

// ProbeResult holds the outcome of a single probe execution
type ProbeResult struct {
	ProbeName  string         `json:"probe_name"`
	ProbeType  string         `json:"probe_type"`
	Mode       domain.ProbeMode `json:"mode"`
	Passed     bool           `json:"passed"`
	Detail     map[string]any `json:"detail,omitempty"`
	Error      *string        `json:"error,omitempty"`
	ExecutedAt time.Time      `json:"executed_at"`
}

// Probe is the interface all probe implementations must satisfy
type Probe interface {
	// Execute runs the probe and returns the result
	Execute(ctx context.Context) (*ProbeResult, error)
	// Name returns the probe's identifier
	Name() string
	// Type returns the probe type (http, cmd, k8s, prometheus)
	Type() string
	// Mode returns when this probe should fire
	Mode() domain.ProbeMode
}

// SafeExecute runs a probe with error handling; it never returns an error
func SafeExecute(ctx context.Context, p Probe) *ProbeResult {
	result, err := p.Execute(ctx)
	if err != nil {
		log.Printf("Probe %s failed: %v", p.Name(), err)
		errStr := err.Error()
		return &ProbeResult{
			ProbeName:  p.Name(),
			ProbeType:  p.Type(),
			Mode:       p.Mode(),
			Passed:     false,
			Error:      &errStr,
			ExecutedAt: time.Now().UTC(),
		}
	}
	return result
}
