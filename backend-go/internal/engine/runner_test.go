package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaosduck/backend-go/internal/safety"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractStringSliceFromStringSlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []string{"i-111", "i-222", "i-333"},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{"i-111", "i-222", "i-333"}, result)
}

func TestExtractStringSliceFromAnySlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []any{"i-aaa", "i-bbb"},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{"i-aaa", "i-bbb"}, result)
}

func TestExtractStringSliceFromAnySliceMixed(t *testing.T) {
	params := map[string]any{
		"items": []any{"valid", 42, "also-valid"},
	}
	result := extractStringSlice(params, "items")
	// Non-string items should be skipped
	assert.Equal(t, []string{"valid", "also-valid"}, result)
}

func TestExtractStringSliceMissingKey(t *testing.T) {
	params := map[string]any{
		"other": "value",
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Nil(t, result)
}

func TestExtractStringSliceWrongType(t *testing.T) {
	params := map[string]any{
		"instance_ids": "not-a-slice",
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Nil(t, result)
}

func TestExtractStringSliceEmptySlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []string{},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{}, result)
}

func TestExtractStringSliceNilMap(t *testing.T) {
	result := extractStringSlice(nil, "key")
	assert.Nil(t, result)
}

func TestCallAISuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/review-steady-state", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.NotNil(t, body["steady_state"])

		w.WriteHeader(200)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"review": "System looks healthy",
			"score":  0.95,
		}))
	}))
	defer srv.Close()

	runner := NewRunner(nil, nil,
		safety.NewEmergencyStopManager(),
		safety.NewRollbackManager(),
		safety.NewSnapshotManager(nil),
		nil, srv.URL,
	)

	result, err := runner.callAI("/review-steady-state", map[string]any{
		"steady_state": map[string]any{"pods": 3},
	})
	require.NoError(t, err)
	assert.Equal(t, "System looks healthy", result["review"])
}

func TestCallAIServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"detail":"internal error"}`))
	}))
	defer srv.Close()

	runner := NewRunner(nil, nil,
		safety.NewEmergencyStopManager(),
		safety.NewRollbackManager(),
		safety.NewSnapshotManager(nil),
		nil, srv.URL,
	)

	_, err := runner.callAI("/analyze", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestCallAINoURL(t *testing.T) {
	runner := NewRunner(nil, nil,
		safety.NewEmergencyStopManager(),
		safety.NewRollbackManager(),
		safety.NewSnapshotManager(nil),
		nil, "",
	)

	_, err := runner.callAI("/analyze", map[string]any{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestCallAIConnectionRefused(t *testing.T) {
	runner := NewRunner(nil, nil,
		safety.NewEmergencyStopManager(),
		safety.NewRollbackManager(),
		safety.NewSnapshotManager(nil),
		nil, "http://127.0.0.1:1",
	)

	_, err := runner.callAI("/analyze", map[string]any{})
	assert.Error(t, err)
}
