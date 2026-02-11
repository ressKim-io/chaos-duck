package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/observability"
	"github.com/chaosduck/backend-go/internal/safety"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() (*gin.Engine, *ChaosHandler) {
	gin.SetMode(gin.TestMode)
	metrics := observability.NewMetrics()
	esm := safety.NewEmergencyStopManager()
	rollbackMgr := safety.NewRollbackManager()
	h := NewChaosHandler(nil, nil, esm, rollbackMgr, metrics)
	r := gin.New()
	return r, h
}

func TestStreamExperiment_NoDB(t *testing.T) {
	r, h := setupTestRouter()
	r.GET("/experiments/:experiment_id/stream", h.StreamExperiment)

	req := httptest.NewRequest("GET", "/experiments/test123/stream", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "Database not available", body["detail"])
}

func TestSendSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"status": "running"}
	sendSSE(c, "experiment", data)

	body := w.Body.String()
	assert.Contains(t, body, "event: experiment\n")
	assert.Contains(t, body, `"status":"running"`)
	assert.Contains(t, body, "\n\n")
}

func TestTerminalStatuses(t *testing.T) {
	assert.True(t, terminalStatuses[domain.StatusCompleted])
	assert.True(t, terminalStatuses[domain.StatusFailed])
	assert.True(t, terminalStatuses[domain.StatusRolledBack])
	assert.True(t, terminalStatuses[domain.StatusEmergencyStopped])
	assert.False(t, terminalStatuses[domain.StatusRunning])
	assert.False(t, terminalStatuses[domain.StatusPending])
}
