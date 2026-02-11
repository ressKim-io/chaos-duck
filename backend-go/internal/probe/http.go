package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
)

// HTTPProbe validates that an HTTP endpoint returns the expected status code
// and optionally matches a pattern in the response body
type HTTPProbe struct {
	name           string
	mode           domain.ProbeMode
	url            string
	method         string
	expectedStatus int
	timeout        time.Duration
	bodyPattern    *regexp.Regexp
	headers        map[string]string
}

// HTTPProbeConfig holds construction parameters for HTTPProbe
type HTTPProbeConfig struct {
	Name           string
	Mode           domain.ProbeMode
	URL            string
	Method         string
	ExpectedStatus int
	Timeout        time.Duration
	BodyPattern    string
	Headers        map[string]string
}

// NewHTTPProbe creates an HTTP probe from config
func NewHTTPProbe(cfg HTTPProbeConfig) (*HTTPProbe, error) {
	if cfg.Method == "" {
		cfg.Method = "GET"
	}
	if cfg.ExpectedStatus == 0 {
		cfg.ExpectedStatus = 200
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	var pat *regexp.Regexp
	if cfg.BodyPattern != "" {
		var err error
		pat, err = regexp.Compile(cfg.BodyPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid body pattern: %w", err)
		}
	}

	return &HTTPProbe{
		name:           cfg.Name,
		mode:           cfg.Mode,
		url:            cfg.URL,
		method:         cfg.Method,
		expectedStatus: cfg.ExpectedStatus,
		timeout:        cfg.Timeout,
		bodyPattern:    pat,
		headers:        cfg.Headers,
	}, nil
}

func (p *HTTPProbe) Name() string          { return p.name }
func (p *HTTPProbe) Type() string          { return "http" }
func (p *HTTPProbe) Mode() domain.ProbeMode { return p.mode }

func (p *HTTPProbe) Execute(ctx context.Context) (*ProbeResult, error) {
	client := &http.Client{Timeout: p.timeout}

	req, err := http.NewRequestWithContext(ctx, p.method, p.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	statusOK := resp.StatusCode == p.expectedStatus
	bodyOK := true

	if p.bodyPattern != nil && statusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyOK = p.bodyPattern.Match(body)
	}

	return &ProbeResult{
		ProbeName: p.name,
		ProbeType: "http",
		Mode:      p.mode,
		Passed:    statusOK && bodyOK,
		Detail: map[string]any{
			"url":             p.url,
			"status_code":     resp.StatusCode,
			"expected_status": p.expectedStatus,
			"body_match":      bodyOK,
			"response_time_ms": elapsed.Milliseconds(),
		},
		ExecutedAt: time.Now().UTC(),
	}, nil
}
