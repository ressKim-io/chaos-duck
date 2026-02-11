package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chaosduck/backend-go/internal/config"
	"github.com/chaosduck/backend-go/internal/db"
	"github.com/chaosduck/backend-go/internal/engine"
	"github.com/chaosduck/backend-go/internal/handler"
	"github.com/chaosduck/backend-go/internal/observability"
	"github.com/chaosduck/backend-go/internal/safety"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	// Database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("Warning: database not available: %v", err)
	}
	var queries *db.Queries
	if pool != nil {
		queries = db.New(pool)
		defer pool.Close()
	}

	// Safety stack
	esm := safety.NewEmergencyStopManager()
	rollbackMgr := safety.NewRollbackManager()
	snapshotMgr := safety.NewSnapshotManager(queries)

	// Engines (fail gracefully if not available)
	var k8sEngine *engine.K8sEngine
	k8sEngine, err = engine.NewK8sEngine(cfg.KubeConfig, esm)
	if err != nil {
		log.Printf("Warning: K8s engine not available: %v", err)
		k8sEngine = nil
	}

	var awsEngine *engine.AwsEngine
	awsEngine, err = engine.NewAwsEngine(ctx, cfg.AWSRegion, esm)
	if err != nil {
		log.Printf("Warning: AWS engine not available: %v", err)
		awsEngine = nil
	}

	// Runner
	runner := engine.NewRunner(k8sEngine, awsEngine, esm, rollbackMgr, snapshotMgr, queries, cfg.AIServiceURL)

	// Metrics
	metrics := observability.NewMetrics()

	// Handlers
	chaosHandler := handler.NewChaosHandler(runner, queries, esm, rollbackMgr, metrics)
	topoHandler := handler.NewTopologyHandler(k8sEngine, awsEngine)
	analysisHandler := handler.NewAnalysisHandler(queries, cfg.AIServiceURL)

	// Router
	r := handler.SetupRouter(chaosHandler, topoHandler, analysisHandler, esm, metrics, cfg.CORSAllowOrigin)

	// Server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Printf("ChaosDuck backend-go starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down... triggering emergency stop")
	esm.Trigger()
	rollbackMgr.RollbackAll()

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	log.Println("Server stopped")
}
