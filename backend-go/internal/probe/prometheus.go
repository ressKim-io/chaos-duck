package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
)

// PromProbe executes a PromQL query against a Prometheus endpoint
// and compares the result against a threshold
type PromProbe struct {
	name       string
	mode       domain.ProbeMode
	endpoint   string
	query      string
	comparator string
	threshold  float64
	timeout    time.Duration
	client     *http.Client
}

// PromProbeConfig holds construction parameters for PromProbe
type PromProbeConfig struct {
	Name       string
	Mode       domain.ProbeMode
	Endpoint   string
	Query      string
	Comparator string
	Threshold  float64
	Timeout    time.Duration
}

// NewPromProbe creates a Prometheus query probe
func NewPromProbe(cfg PromProbeConfig) *PromProbe {
	if cfg.Comparator == "" {
		cfg.Comparator = ">"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &PromProbe{
		name:       cfg.Name,
		mode:       cfg.Mode,
		endpoint:   strings.TrimRight(cfg.Endpoint, "/"),
		query:      cfg.Query,
		comparator: cfg.Comparator,
		threshold:  cfg.Threshold,
		timeout:    cfg.Timeout,
		client:     &http.Client{Timeout: cfg.Timeout},
	}
}

func (p *PromProbe) Name() string          { return p.name }
func (p *PromProbe) Type() string          { return "prometheus" }
func (p *PromProbe) Mode() domain.ProbeMode { return p.mode }

func (p *PromProbe) Execute(ctx context.Context) (*ProbeResult, error) {
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", p.endpoint, url.QueryEscape(p.query))
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prometheus request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			Result []struct {
				Value [2]json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(body.Data.Result) == 0 {
		return &ProbeResult{
			ProbeName: p.name,
			ProbeType: "prometheus",
			Mode:      p.mode,
			Passed:    false,
			Detail: map[string]any{
				"query": p.query,
				"error": "No results returned",
			},
			ExecutedAt: time.Now().UTC(),
		}, nil
	}

	// Parse the first result's value (index 1 is the actual value)
	var valStr string
	if err := json.Unmarshal(body.Data.Result[0].Value[1], &valStr); err != nil {
		return nil, fmt.Errorf("parse value: %w", err)
	}
	value, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parse float value: %w", err)
	}

	passed := p.compare(value)

	return &ProbeResult{
		ProbeName: p.name,
		ProbeType: "prometheus",
		Mode:      p.mode,
		Passed:    passed,
		Detail: map[string]any{
			"query":        p.query,
			"value":        value,
			"comparator":   p.comparator,
			"threshold":    p.threshold,
			"result_count": len(body.Data.Result),
		},
		ExecutedAt: time.Now().UTC(),
	}, nil
}

func (p *PromProbe) compare(value float64) bool {
	switch p.comparator {
	case ">":
		return value > p.threshold
	case ">=":
		return value >= p.threshold
	case "<":
		return value < p.threshold
	case "<=":
		return value <= p.threshold
	case "==":
		return value == p.threshold
	case "!=":
		return value != p.threshold
	default:
		return false
	}
}
