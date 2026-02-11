package probe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
)

// mockProbe for testing SafeExecute
type testProbe struct {
	name   string
	result *ProbeResult
	err    error
}

func (p *testProbe) Execute(ctx context.Context) (*ProbeResult, error) {
	return p.result, p.err
}
func (p *testProbe) Name() string           { return p.name }
func (p *testProbe) Type() string           { return "test" }
func (p *testProbe) Mode() domain.ProbeMode { return domain.ProbeModeSOT }

func TestSafeExecuteSuccess(t *testing.T) {
	p := &testProbe{
		name: "ok-probe",
		result: &ProbeResult{
			ProbeName:  "ok-probe",
			ProbeType:  "test",
			Mode:       domain.ProbeModeSOT,
			Passed:     true,
			ExecutedAt: time.Now().UTC(),
		},
	}

	result := SafeExecute(context.Background(), p)
	assert.True(t, result.Passed)
	assert.Equal(t, "ok-probe", result.ProbeName)
	assert.Nil(t, result.Error)
}

func TestSafeExecuteError(t *testing.T) {
	p := &testProbe{
		name: "err-probe",
		err:  errors.New("connection refused"),
	}

	result := SafeExecute(context.Background(), p)
	assert.False(t, result.Passed)
	assert.Equal(t, "err-probe", result.ProbeName)
	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "connection refused")
}
