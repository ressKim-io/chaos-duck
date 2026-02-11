package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// AnalysisHandler proxies AI analysis requests to the Python AI microservice
type AnalysisHandler struct {
	queries      *db.Queries
	aiServiceURL string
	httpClient   *http.Client
}

// NewAnalysisHandler creates a new AnalysisHandler
func NewAnalysisHandler(queries *db.Queries, aiServiceURL string) *AnalysisHandler {
	return &AnalysisHandler{
		queries:      queries,
		aiServiceURL: aiServiceURL,
		httpClient:   &http.Client{Timeout: 60 * time.Second},
	}
}

// AnalyzeExperiment proxies to AI service for experiment analysis
func (h *AnalysisHandler) AnalyzeExperiment(c *gin.Context) {
	experimentID := c.Param("experiment_id")

	rec, err := h.queries.GetExperiment(c.Request.Context(), experimentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Experiment not found"})
		return
	}

	result := recordToResult(rec)

	body := map[string]any{
		"experiment_data": result,
		"steady_state":    result.SteadyState,
		"observations":    result.Observations,
	}

	resp, err := h.proxyToAI("/analyze", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}

	// Persist analysis result if we got one
	if severity, ok := resp["severity"].(string); ok {
		rootCause, _ := resp["root_cause"].(string)
		confidence, _ := resp["confidence"].(float64)
		resilienceScore, _ := resp["resilience_score"].(float64)
		recsJSON, _ := json.Marshal(resp["recommendations"])

		h.queries.CreateAnalysisResult(c.Request.Context(), db.CreateAnalysisResultParams{
			ExperimentID:    experimentID,
			Severity:        severity,
			RootCause:       rootCause,
			Confidence:      confidence,
			Recommendations: recsJSON,
			ResilienceScore: pgtype.Float8{Float64: resilienceScore, Valid: true},
		})
	}

	c.JSON(http.StatusOK, resp)
}

// GenerateHypotheses proxies to AI service
func (h *AnalysisHandler) GenerateHypotheses(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	resp, err := h.proxyToAI("/hypotheses", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CalculateResilienceScore proxies to AI service
func (h *AnalysisHandler) CalculateResilienceScore(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	resp, err := h.proxyToAI("/resilience-score", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GenerateReport proxies to AI service
func (h *AnalysisHandler) GenerateReport(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	resp, err := h.proxyToAI("/report", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GenerateExperiments proxies to AI service
func (h *AnalysisHandler) GenerateExperiments(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	resp, err := h.proxyToAI("/generate-experiments", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// NLExperiment proxies natural language experiment creation to AI service
func (h *AnalysisHandler) NLExperiment(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	text, _ := body["text"].(string)
	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Text is required"})
		return
	}

	resp, err := h.proxyToAI("/nl-experiment", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ResilienceTrend returns resilience score trend from DB
func (h *AnalysisHandler) ResilienceTrend(c *gin.Context) {
	namespace := c.Query("namespace")
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 30
	}

	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	sinceTS := pgtype.Timestamptz{Time: since, Valid: true}

	var records []db.AnalysisResult
	if namespace != "" {
		records, err = h.queries.ListAnalysisResultsSinceByNamespace(c.Request.Context(), db.ListAnalysisResultsSinceByNamespaceParams{
			Since:     sinceTS,
			Namespace: namespace,
		})
	} else {
		records, err = h.queries.ListAnalysisResultsSince(c.Request.Context(), sinceTS)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	trend := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"experiment_id":   r.ExperimentID,
			"severity":        r.Severity,
		}
		if r.ResilienceScore.Valid {
			entry["resilience_score"] = r.ResilienceScore.Float64
		}
		if r.CreatedAt.Valid {
			entry["created_at"] = r.CreatedAt.Time.Format(time.RFC3339)
		}
		trend = append(trend, entry)
	}

	c.JSON(http.StatusOK, gin.H{
		"trend":       trend,
		"count":       len(trend),
		"period_days": days,
		"namespace":   namespace,
	})
}

// ResilienceTrendSummary returns AI-generated summary of the trend
func (h *AnalysisHandler) ResilienceTrendSummary(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 30
	}

	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	sinceTS := pgtype.Timestamptz{Time: since, Valid: true}

	records, err := h.queries.ListAnalysisResultsSince(c.Request.Context(), sinceTS)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	experimentsData := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"experiment_id": r.ExperimentID,
			"severity":      r.Severity,
			"root_cause":    r.RootCause,
		}
		if r.ResilienceScore.Valid {
			entry["resilience_score"] = r.ResilienceScore.Float64
		}
		experimentsData = append(experimentsData, entry)
	}

	body := map[string]any{"experiments": experimentsData}
	resp, err := h.proxyToAI("/resilience-score", body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("AI service error: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":     resp,
		"data_points": len(records),
		"period_days": days,
	})
}

// proxyToAI sends a JSON POST request to the AI microservice
func (h *AnalysisHandler) proxyToAI(path string, body any) (map[string]any, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	resp, err := h.httpClient.Post(
		h.aiServiceURL+path,
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result, nil
}
