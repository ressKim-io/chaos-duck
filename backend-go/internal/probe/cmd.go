package probe

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
)

// CmdProbe executes a shell command and validates exit code and output
type CmdProbe struct {
	name             string
	mode             domain.ProbeMode
	command          string
	expectedExitCode int
	outputContains   string
	timeout          time.Duration
}

// CmdProbeConfig holds construction parameters for CmdProbe
type CmdProbeConfig struct {
	Name             string
	Mode             domain.ProbeMode
	Command          string
	ExpectedExitCode int
	OutputContains   string
	Timeout          time.Duration
}

// NewCmdProbe creates a command probe from config
func NewCmdProbe(cfg CmdProbeConfig) *CmdProbe {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &CmdProbe{
		name:             cfg.Name,
		mode:             cfg.Mode,
		command:          cfg.Command,
		expectedExitCode: cfg.ExpectedExitCode,
		outputContains:   cfg.OutputContains,
		timeout:          cfg.Timeout,
	}
}

func (p *CmdProbe) Name() string          { return p.name }
func (p *CmdProbe) Type() string          { return "cmd" }
func (p *CmdProbe) Mode() domain.ProbeMode { return p.mode }

func (p *CmdProbe) Execute(ctx context.Context) (*ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", p.command)
	output, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if ctx.Err() != nil {
			errStr := fmt.Sprintf("Command timed out after %v", p.timeout)
			return &ProbeResult{
				ProbeName:  p.name,
				ProbeType:  "cmd",
				Mode:       p.mode,
				Passed:     false,
				Error:      &errStr,
				ExecutedAt: time.Now().UTC(),
			}, nil
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	exitOK := exitCode == p.expectedExitCode
	outputOK := true
	stdout := string(output)
	if p.outputContains != "" && exitOK {
		outputOK = strings.Contains(stdout, p.outputContains)
	}

	// Truncate output for storage
	if len(stdout) > 500 {
		stdout = stdout[:500]
	}

	return &ProbeResult{
		ProbeName: p.name,
		ProbeType: "cmd",
		Mode:      p.mode,
		Passed:    exitOK && outputOK,
		Detail: map[string]any{
			"command":            p.command,
			"exit_code":          exitCode,
			"expected_exit_code": p.expectedExitCode,
			"stdout":             stdout,
			"output_match":       outputOK,
		},
		ExecutedAt: time.Now().UTC(),
	}, nil
}
