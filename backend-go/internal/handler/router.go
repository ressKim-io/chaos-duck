package handler

import (
	"net/http"

	"github.com/chaosduck/backend-go/internal/observability"
	"github.com/chaosduck/backend-go/internal/safety"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRouter configures all API routes
func SetupRouter(
	chaos *ChaosHandler,
	topology *TopologyHandler,
	analysis *AnalysisHandler,
	esm *safety.EmergencyStopManager,
	metrics *observability.Metrics,
	corsOrigin string,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware(corsOrigin))
	r.Use(PrometheusMiddleware(metrics))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"emergency_stop": esm.IsTriggered(),
		})
	})

	// Prometheus metrics
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Emergency stop
	r.POST("/emergency-stop", func(c *gin.Context) {
		esm.Trigger()
		c.JSON(http.StatusOK, gin.H{"status": "emergency_stop_triggered"})
	})

	// Chaos endpoints
	chaosGroup := r.Group("/api/chaos")
	{
		chaosGroup.POST("/experiments", chaos.CreateExperiment)
		chaosGroup.GET("/experiments", chaos.ListExperiments)
		chaosGroup.GET("/experiments/:experiment_id", chaos.GetExperiment)
		chaosGroup.POST("/experiments/:experiment_id/rollback", chaos.RollbackExperiment)
		chaosGroup.POST("/dry-run", chaos.DryRun)
	}

	// Topology endpoints
	topoGroup := r.Group("/api/topology")
	{
		topoGroup.GET("/k8s", topology.GetK8sTopology)
		topoGroup.GET("/aws", topology.GetAWSTopology)
		topoGroup.GET("/combined", topology.GetCombinedTopology)
		topoGroup.GET("/steady-state", topology.GetSteadyState)
	}

	// Analysis endpoints (proxy to AI service)
	analysisGroup := r.Group("/api/analysis")
	{
		analysisGroup.POST("/experiment/:experiment_id", analysis.AnalyzeExperiment)
		analysisGroup.POST("/hypotheses", analysis.GenerateHypotheses)
		analysisGroup.POST("/resilience-score", analysis.CalculateResilienceScore)
		analysisGroup.POST("/report", analysis.GenerateReport)
		analysisGroup.POST("/generate-experiments", analysis.GenerateExperiments)
		analysisGroup.POST("/nl-experiment", analysis.NLExperiment)
		analysisGroup.GET("/resilience-trend", analysis.ResilienceTrend)
		analysisGroup.GET("/resilience-trend/summary", analysis.ResilienceTrendSummary)
	}

	return r
}
