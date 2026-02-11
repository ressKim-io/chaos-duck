package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPProbeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name: "health",
		Mode: domain.ProbeModeSOT,
		URL:  srv.URL,
	})
	require.NoError(t, err)

	assert.Equal(t, "health", p.Name())
	assert.Equal(t, "http", p.Type())
	assert.Equal(t, domain.ProbeModeSOT, p.Mode())

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
	assert.Equal(t, 200, result.Detail["status_code"])
	assert.Equal(t, 200, result.Detail["expected_status"])
	assert.Equal(t, true, result.Detail["body_match"])
}

func TestHTTPProbeWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name: "failing",
		Mode: domain.ProbeModeSOT,
		URL:  srv.URL,
	})
	require.NoError(t, err)

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.False(t, result.Passed)
	assert.Equal(t, 500, result.Detail["status_code"])
}

func TestHTTPProbeBodyPattern(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"healthy","version":"1.2.3"}`))
	}))
	defer srv.Close()

	// Matching pattern
	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name:        "body-match",
		Mode:        domain.ProbeModeSOT,
		URL:         srv.URL,
		BodyPattern: `"status":"healthy"`,
	})
	require.NoError(t, err)

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)

	// Non-matching pattern
	p2, err := NewHTTPProbe(HTTPProbeConfig{
		Name:        "body-no-match",
		Mode:        domain.ProbeModeSOT,
		URL:         srv.URL,
		BodyPattern: `"status":"down"`,
	})
	require.NoError(t, err)

	result2, err := p2.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result2.Passed)
}

func TestHTTPProbeCustomStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()

	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name:           "created",
		Mode:           domain.ProbeModeSOT,
		URL:            srv.URL,
		ExpectedStatus: 201,
	})
	require.NoError(t, err)

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestHTTPProbeTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name:    "slow",
		Mode:    domain.ProbeModeSOT,
		URL:     srv.URL,
		Timeout: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	_, err = p.Execute(context.Background())
	assert.Error(t, err) // Should fail due to timeout
}

func TestHTTPProbeInvalidPattern(t *testing.T) {
	_, err := NewHTTPProbe(HTTPProbeConfig{
		Name:        "bad-regex",
		Mode:        domain.ProbeModeSOT,
		URL:         "http://localhost",
		BodyPattern: "[invalid",
	})
	assert.Error(t, err)
}

func TestHTTPProbeConnectionRefused(t *testing.T) {
	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name:    "unreachable",
		Mode:    domain.ProbeModeSOT,
		URL:     "http://127.0.0.1:1",
		Timeout: 500 * time.Millisecond,
	})
	require.NoError(t, err)

	_, err = p.Execute(context.Background())
	assert.Error(t, err)
}

func TestHTTPProbeResponseTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p, err := NewHTTPProbe(HTTPProbeConfig{
		Name: "timing",
		Mode: domain.ProbeModeSOT,
		URL:  srv.URL,
	})
	require.NoError(t, err)

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	responseTime, ok := result.Detail["response_time_ms"]
	assert.True(t, ok)
	assert.GreaterOrEqual(t, responseTime, int64(0))
}
