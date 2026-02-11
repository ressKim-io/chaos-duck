package probe

import (
	"context"
	"testing"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdProbeSuccess(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:    "echo-test",
		Mode:    domain.ProbeModeSOT,
		Command: "echo hello",
	})

	assert.Equal(t, "echo-test", p.Name())
	assert.Equal(t, "cmd", p.Type())
	assert.Equal(t, domain.ProbeModeSOT, p.Mode())

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
	assert.Equal(t, 0, result.Detail["exit_code"])
	assert.Contains(t, result.Detail["stdout"], "hello")
}

func TestCmdProbeNonZeroExit(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:    "false-test",
		Mode:    domain.ProbeModeSOT,
		Command: "false",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.False(t, result.Passed)
	assert.NotEqual(t, 0, result.Detail["exit_code"])
}

func TestCmdProbeExpectedNonZero(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:             "expect-fail",
		Mode:             domain.ProbeModeSOT,
		Command:          "false",
		ExpectedExitCode: 1,
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
}

func TestCmdProbeOutputContains(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:           "output-match",
		Mode:           domain.ProbeModeSOT,
		Command:        "echo 'hello world'",
		OutputContains: "world",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, true, result.Detail["output_match"])
}

func TestCmdProbeOutputNotContains(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:           "output-no-match",
		Mode:           domain.ProbeModeSOT,
		Command:        "echo 'hello'",
		OutputContains: "missing",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestCmdProbeTimeout(t *testing.T) {
	p := NewCmdProbe(CmdProbeConfig{
		Name:    "slow-cmd",
		Mode:    domain.ProbeModeSOT,
		Command: "sleep 10",
		Timeout: 500 * time.Millisecond,
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.False(t, result.Passed)
	assert.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "timed out")
}
