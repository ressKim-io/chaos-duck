package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromProbeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/v1/query")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"data": {
				"result": [
					{"value": [1234567890, "0.95"]}
				]
			}
		}`))
	}))
	defer srv.Close()

	p := NewPromProbe(PromProbeConfig{
		Name:       "latency-check",
		Mode:       domain.ProbeModeSOT,
		Endpoint:   srv.URL,
		Query:      "up",
		Comparator: ">",
		Threshold:  0.5,
	})

	assert.Equal(t, "latency-check", p.Name())
	assert.Equal(t, "prometheus", p.Type())
	assert.Equal(t, domain.ProbeModeSOT, p.Mode())

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
	assert.Equal(t, 0.95, result.Detail["value"])
	assert.Equal(t, ">", result.Detail["comparator"])
	assert.Equal(t, 0.5, result.Detail["threshold"])
	assert.Equal(t, 1, result.Detail["result_count"])
}

func TestPromProbeFailsThreshold(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{
			"data": {
				"result": [
					{"value": [1234567890, "0.3"]}
				]
			}
		}`))
	}))
	defer srv.Close()

	p := NewPromProbe(PromProbeConfig{
		Name:       "low-value",
		Mode:       domain.ProbeModeSOT,
		Endpoint:   srv.URL,
		Query:      "up",
		Comparator: ">",
		Threshold:  0.5,
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestPromProbeNoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": {"result": []}}`))
	}))
	defer srv.Close()

	p := NewPromProbe(PromProbeConfig{
		Name:     "empty",
		Mode:     domain.ProbeModeSOT,
		Endpoint: srv.URL,
		Query:    "nonexistent_metric",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, "No results returned", result.Detail["error"])
}

func TestPromProbeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	p := NewPromProbe(PromProbeConfig{
		Name:     "server-err",
		Mode:     domain.ProbeModeSOT,
		Endpoint: srv.URL,
		Query:    "up",
	})

	_, err := p.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestPromProbeComparators(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data": {"result": [{"value": [0, "5.0"]}]}}`))
	}))
	defer srv.Close()

	tests := []struct {
		comparator string
		threshold  float64
		expected   bool
	}{
		{">", 3.0, true},
		{">", 5.0, false},
		{">=", 5.0, true},
		{">=", 6.0, false},
		{"<", 10.0, true},
		{"<", 3.0, false},
		{"<=", 5.0, true},
		{"<=", 4.0, false},
		{"==", 5.0, true},
		{"==", 4.0, false},
		{"!=", 4.0, true},
		{"!=", 5.0, false},
		{"invalid", 5.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.comparator, func(t *testing.T) {
			p := NewPromProbe(PromProbeConfig{
				Name:       "cmp-test",
				Mode:       domain.ProbeModeSOT,
				Endpoint:   srv.URL,
				Query:      "up",
				Comparator: tt.comparator,
				Threshold:  tt.threshold,
			})

			result, err := p.Execute(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.Passed, "comparator=%s threshold=%f", tt.comparator, tt.threshold)
		})
	}
}

func TestPromProbeDefaultComparator(t *testing.T) {
	p := NewPromProbe(PromProbeConfig{
		Name:     "default-cmp",
		Mode:     domain.ProbeModeSOT,
		Endpoint: "http://localhost:9090",
		Query:    "up",
	})
	// Default comparator should be ">"
	assert.Equal(t, ">", p.comparator)
}

func TestPromProbeConnectionRefused(t *testing.T) {
	p := NewPromProbe(PromProbeConfig{
		Name:     "unreachable",
		Mode:     domain.ProbeModeSOT,
		Endpoint: "http://127.0.0.1:1",
		Query:    "up",
	})

	_, err := p.Execute(context.Background())
	assert.Error(t, err)
}
