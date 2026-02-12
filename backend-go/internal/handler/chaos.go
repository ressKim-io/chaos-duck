package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/engine"
	"github.com/chaosduck/backend-go/internal/observability"
	"github.com/chaosduck/backend-go/internal/safety"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ChaosHandler handles chaos experiment endpoints
type ChaosHandler struct {
	runner      *engine.Runner
	queries     *db.Queries
	esm         *safety.EmergencyStopManager
	rollbackMgr *safety.RollbackManager
	metrics     *observability.Metrics
}

// NewChaosHandler creates a new ChaosHandler
func NewChaosHandler(
	runner *engine.Runner,
	queries *db.Queries,
	esm *safety.EmergencyStopManager,
	rollbackMgr *safety.RollbackManager,
	metrics *observability.Metrics,
) *ChaosHandler {
	return &ChaosHandler{
		runner:      runner,
		queries:     queries,
		esm:         esm,
		rollbackMgr: rollbackMgr,
		metrics:     metrics,
	}
}

// CreateExperiment creates and runs a chaos experiment
func (h *ChaosHandler) CreateExperiment(c *gin.Context) {
	if h.esm.IsTriggered() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Emergency stop is active"})
		return
	}

	var cfg domain.ExperimentConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// Fill in zero-value safety fields with defaults
	defaults := domain.DefaultSafetyConfig()
	if cfg.Safety.TimeoutSeconds == 0 {
		cfg.Safety.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if cfg.Safety.MaxBlastRadius == 0 {
		cfg.Safety.MaxBlastRadius = defaults.MaxBlastRadius
	}
	if cfg.Safety.HealthCheckInterval == 0 {
		cfg.Safety.HealthCheckInterval = defaults.HealthCheckInterval
	}
	if cfg.Safety.HealthCheckFailureThreshold == 0 {
		cfg.Safety.HealthCheckFailureThreshold = defaults.HealthCheckFailureThreshold
	}

	experimentID := uuid.New().String()[:8]
	now := time.Now().UTC()

	// Persist initial record
	if h.queries != nil {
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			log.Printf("Failed to marshal config for experiment %s: %v", experimentID, err)
			configJSON = []byte("{}")
		}
		if _, err := h.queries.CreateExperiment(c.Request.Context(), db.CreateExperimentParams{
			ID:     experimentID,
			Config: configJSON,
			Status: string(domain.StatusRunning),
			Phase:  string(domain.PhaseSteadyState),
			StartedAt: pgtype.Timestamptz{
				Time:  now,
				Valid: true,
			},
		}); err != nil {
			log.Printf("Failed to persist experiment %s: %v", experimentID, err)
		}
	}

	h.metrics.RecordExperimentStart()

	result, err := h.runner.Run(c.Request.Context(), experimentID, cfg)
	if err != nil {
		duration := time.Since(now).Seconds()
		h.metrics.RecordExperimentEnd(string(cfg.ChaosType), "failed", duration)
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	duration := time.Since(now).Seconds()
	h.metrics.RecordExperimentEnd(string(cfg.ChaosType), string(result.Status), duration)
	c.JSON(http.StatusOK, result)
}

// ListExperiments returns all experiments
func (h *ChaosHandler) ListExperiments(c *gin.Context) {
	if h.queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Database not available"})
		return
	}
	records, err := h.queries.ListExperiments(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	results := make([]domain.ExperimentResult, 0, len(records))
	for _, rec := range records {
		results = append(results, recordToResult(rec))
	}
	c.JSON(http.StatusOK, results)
}

// GetExperiment returns a specific experiment
func (h *ChaosHandler) GetExperiment(c *gin.Context) {
	if h.queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Database not available"})
		return
	}
	experimentID := c.Param("experiment_id")

	rec, err := h.queries.GetExperiment(c.Request.Context(), experimentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Experiment not found"})
		return
	}

	c.JSON(http.StatusOK, recordToResult(rec))
}

// RollbackExperiment triggers rollback for a specific experiment
func (h *ChaosHandler) RollbackExperiment(c *gin.Context) {
	experimentID := c.Param("experiment_id")

	if h.queries != nil {
		_, err := h.queries.GetExperiment(c.Request.Context(), experimentID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"detail": "Experiment not found"})
			return
		}
	}

	results := h.rollbackMgr.Rollback(experimentID)
	if h.queries != nil {
		if err := h.queries.UpdateExperimentStatus(c.Request.Context(), db.UpdateExperimentStatusParams{
			ID:     experimentID,
			Status: string(domain.StatusRolledBack),
		}); err != nil {
			log.Printf("Failed to update experiment status: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"experiment_id":    experimentID,
		"rollback_results": results,
	})
}

// DryRun executes a dry-run chaos experiment
func (h *ChaosHandler) DryRun(c *gin.Context) {
	var cfg domain.ExperimentConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	cfg.Safety.DryRun = true
	defaults := domain.DefaultSafetyConfig()
	if cfg.Safety.TimeoutSeconds == 0 {
		cfg.Safety.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if cfg.Safety.MaxBlastRadius == 0 {
		cfg.Safety.MaxBlastRadius = defaults.MaxBlastRadius
	}

	experimentID := "dry-" + uuid.New().String()[:8]
	now := time.Now().UTC()

	result := domain.ExperimentResult{
		ExperimentID: experimentID,
		Config:       cfg,
		Status:       domain.StatusCompleted,
		StartedAt:    &now,
		CompletedAt:  &now,
	}

	c.JSON(http.StatusOK, result)
}

// recordToResult converts a DB record to domain ExperimentResult
func recordToResult(rec db.Experiment) domain.ExperimentResult {
	result := domain.ExperimentResult{
		ExperimentID: rec.ID,
		Status:       domain.ExperimentStatus(rec.Status),
		Phase:        domain.ExperimentPhase(rec.Phase),
	}

	// Parse config
	if len(rec.Config) > 0 {
		if err := json.Unmarshal(rec.Config, &result.Config); err != nil {
			log.Printf("Failed to unmarshal config for experiment %s: %v", rec.ID, err)
		}
	}

	if rec.StartedAt.Valid {
		t := rec.StartedAt.Time
		result.StartedAt = &t
	}
	if rec.CompletedAt.Valid {
		t := rec.CompletedAt.Time
		result.CompletedAt = &t
	}
	if len(rec.SteadyState) > 0 {
		var ss map[string]any
		if err := json.Unmarshal(rec.SteadyState, &ss); err != nil {
			log.Printf("Failed to unmarshal steady_state for experiment %s: %v", rec.ID, err)
		}
		result.SteadyState = ss
	}
	if rec.Hypothesis.Valid {
		result.Hypothesis = &rec.Hypothesis.String
	}
	if len(rec.InjectionResult) > 0 {
		var ir map[string]any
		if err := json.Unmarshal(rec.InjectionResult, &ir); err != nil {
			log.Printf("Failed to unmarshal injection_result for experiment %s: %v", rec.ID, err)
		}
		result.InjectionResult = ir
	}
	if len(rec.Observations) > 0 {
		var obs map[string]any
		if err := json.Unmarshal(rec.Observations, &obs); err != nil {
			log.Printf("Failed to unmarshal observations for experiment %s: %v", rec.ID, err)
		}
		result.Observations = obs
	}
	if len(rec.RollbackResult) > 0 {
		var rr map[string]any
		if err := json.Unmarshal(rec.RollbackResult, &rr); err != nil {
			log.Printf("Failed to unmarshal rollback_result for experiment %s: %v", rec.ID, err)
		}
		result.RollbackResult = rr
	}
	if rec.Error.Valid {
		result.Error = &rec.Error.String
	}
	if len(rec.AiInsights) > 0 {
		var ai map[string]any
		if err := json.Unmarshal(rec.AiInsights, &ai); err != nil {
			log.Printf("Failed to unmarshal ai_insights for experiment %s: %v", rec.ID, err)
		}
		result.AIInsights = ai
	}

	return result
}

// terminalStatuses defines statuses that end the SSE stream
var terminalStatuses = map[domain.ExperimentStatus]bool{
	domain.StatusCompleted:        true,
	domain.StatusFailed:           true,
	domain.StatusRolledBack:       true,
	domain.StatusEmergencyStopped: true,
}

// sendSSE writes a single SSE event to the response writer
func sendSSE(c *gin.Context, event string, data any) {
	j, err := json.Marshal(data)
	if err != nil {
		log.Printf("SSE marshal error: %v", err)
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, j)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
}

// StreamExperiment streams experiment updates via Server-Sent Events
func (h *ChaosHandler) StreamExperiment(c *gin.Context) {
	if h.queries == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"detail": "Database not available"})
		return
	}
	experimentID := c.Param("experiment_id")

	// Fetch initial state (also verifies experiment exists)
	rec, err := h.queries.GetExperiment(c.Request.Context(), experimentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Experiment not found"})
		return
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	// Send initial state immediately
	result := recordToResult(rec)
	lastStatus := string(result.Status)
	lastPhase := string(result.Phase)
	sendSSE(c, "experiment", result)

	if terminalStatuses[result.Status] {
		sendSSE(c, "done", gin.H{"status": result.Status})
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	maxTimeout := time.After(5 * time.Minute)

	for {
		select {
		case <-maxTimeout:
			sendSSE(c, "timeout", gin.H{"message": "stream max timeout reached"})
			return
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			rec, err := h.queries.GetExperiment(c.Request.Context(), experimentID)
			if err != nil {
				continue
			}
			result := recordToResult(rec)
			currentStatus := string(result.Status)
			currentPhase := string(result.Phase)

			// Only send when state changes
			if currentStatus != lastStatus || currentPhase != lastPhase {
				lastStatus = currentStatus
				lastPhase = currentPhase
				sendSSE(c, "experiment", result)

				if terminalStatuses[result.Status] {
					sendSSE(c, "done", gin.H{"status": result.Status})
					return
				}
			}
		}
	}
}
