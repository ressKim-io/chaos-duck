package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/safety"
	"github.com/jackc/pgx/v5/pgtype"
)

// Runner orchestrates the 5-phase experiment lifecycle:
// STEADY_STATE -> HYPOTHESIS -> INJECT -> OBSERVE -> ROLLBACK
type Runner struct {
	k8s         *K8sEngine
	aws         *AwsEngine
	esm         *safety.EmergencyStopManager
	rollbackMgr *safety.RollbackManager
	snapshotMgr *safety.SnapshotManager
	queries     *db.Queries
	aiBaseURL   string
}

// NewRunner creates a new experiment runner
func NewRunner(
	k8s *K8sEngine,
	aws *AwsEngine,
	esm *safety.EmergencyStopManager,
	rollbackMgr *safety.RollbackManager,
	snapshotMgr *safety.SnapshotManager,
	queries *db.Queries,
	aiBaseURL string,
) *Runner {
	return &Runner{
		k8s:         k8s,
		aws:         aws,
		esm:         esm,
		rollbackMgr: rollbackMgr,
		snapshotMgr: snapshotMgr,
		queries:     queries,
		aiBaseURL:   aiBaseURL,
	}
}

// Run executes the full 5-phase experiment lifecycle
func (r *Runner) Run(ctx context.Context, experimentID string, cfg domain.ExperimentConfig) (*domain.ExperimentResult, error) {
	if err := r.esm.CheckEmergencyStop(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := &domain.ExperimentResult{
		ExperimentID: experimentID,
		Config:       cfg,
		Status:       domain.StatusRunning,
		Phase:        domain.PhaseSteadyState,
		StartedAt:    &now,
	}
	aiInsights := make(map[string]any)

	// Ensure rollback on panic or error
	defer func() {
		if result.Status == domain.StatusFailed {
			r.rollbackMgr.Rollback(experimentID)
		}
	}()

	// Phase 1: Steady State
	if cfg.TargetNamespace != nil && r.k8s != nil {
		// Capture snapshot
		steadyState, err := r.k8s.GetSteadyState(ctx, *cfg.TargetNamespace)
		if err != nil {
			log.Printf("Steady state capture failed: %v", err)
		} else {
			result.SteadyState = steadyState
			r.snapshotMgr.CaptureK8sSnapshot(ctx, experimentID, *cfg.TargetNamespace, steadyState)
		}
	}

	// Phase 2: Hypothesis (placeholder for AI integration)
	result.Phase = domain.PhaseHypothesis

	// Phase 3: Inject
	result.Phase = domain.PhaseInject
	chaosResult, err := r.executeChaos(ctx, &cfg)
	if err != nil {
		result.Status = domain.StatusFailed
		errStr := err.Error()
		result.Error = &errStr
		r.persistResult(ctx, experimentID, result)
		return result, err
	}
	result.InjectionResult = chaosResult.Result

	if chaosResult.RollbackFn != nil {
		r.rollbackMgr.Push(experimentID, chaosResult.RollbackFn, string(cfg.ChaosType))
	}

	// Phase 4: Observe
	result.Phase = domain.PhaseObserve
	if cfg.TargetNamespace != nil && r.k8s != nil {
		observations, err := r.k8s.GetSteadyState(ctx, *cfg.TargetNamespace)
		if err != nil {
			log.Printf("Observation capture failed: %v", err)
		} else {
			result.Observations = observations
		}
	}

	// Phase 5: Rollback
	result.Phase = domain.PhaseRollback
	result.Status = domain.StatusCompleted
	completedAt := time.Now().UTC()
	result.CompletedAt = &completedAt

	if len(aiInsights) > 0 {
		result.AIInsights = aiInsights
	}

	r.persistResult(ctx, experimentID, result)
	return result, nil
}

// executeChaos routes to the appropriate chaos function based on type
func (r *Runner) executeChaos(ctx context.Context, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	namespace := "default"
	if cfg.TargetNamespace != nil {
		namespace = *cfg.TargetNamespace
	}
	labelSelector := domain.LabelSelectorString(cfg.TargetLabels)

	switch cfg.ChaosType {
	// Kubernetes chaos types
	case domain.ChaosTypePodDelete:
		if r.k8s == nil {
			return nil, fmt.Errorf("k8s engine not available")
		}
		return r.k8s.PodDelete(ctx, namespace, labelSelector, cfg)

	case domain.ChaosTypeNetworkLatency:
		if r.k8s == nil {
			return nil, fmt.Errorf("k8s engine not available")
		}
		latencyMs := 100
		if v, ok := cfg.Parameters["latency_ms"]; ok {
			if f, ok := v.(float64); ok {
				latencyMs = int(f)
			}
		}
		return r.k8s.NetworkLatency(ctx, namespace, labelSelector, latencyMs, cfg)

	case domain.ChaosTypeNetworkLoss:
		if r.k8s == nil {
			return nil, fmt.Errorf("k8s engine not available")
		}
		lossPercent := 10
		if v, ok := cfg.Parameters["loss_percent"]; ok {
			if f, ok := v.(float64); ok {
				lossPercent = int(f)
			}
		}
		return r.k8s.NetworkLoss(ctx, namespace, labelSelector, lossPercent, cfg)

	case domain.ChaosTypeCPUStress:
		if r.k8s == nil {
			return nil, fmt.Errorf("k8s engine not available")
		}
		cores := 1
		if v, ok := cfg.Parameters["cores"]; ok {
			if f, ok := v.(float64); ok {
				cores = int(f)
			}
		}
		return r.k8s.CPUStress(ctx, namespace, labelSelector, cores, cfg.Safety.TimeoutSeconds, cfg)

	case domain.ChaosTypeMemoryStress:
		if r.k8s == nil {
			return nil, fmt.Errorf("k8s engine not available")
		}
		memBytes := "256M"
		if v, ok := cfg.Parameters["memory_bytes"]; ok {
			if s, ok := v.(string); ok {
				memBytes = s
			}
		}
		return r.k8s.MemoryStress(ctx, namespace, labelSelector, memBytes, cfg.Safety.TimeoutSeconds, cfg)

	// AWS chaos types
	case domain.ChaosTypeEC2Stop:
		if r.aws == nil {
			return nil, fmt.Errorf("aws engine not available")
		}
		ids := extractStringSlice(cfg.Parameters, "instance_ids")
		return r.aws.StopEC2(ctx, ids, cfg.Safety.DryRun)

	case domain.ChaosTypeRDSFailover:
		if r.aws == nil {
			return nil, fmt.Errorf("aws engine not available")
		}
		clusterID, _ := cfg.Parameters["db_cluster_id"].(string)
		return r.aws.FailoverRDS(ctx, clusterID, cfg.Safety.DryRun)

	case domain.ChaosTypeRouteBlackhole:
		if r.aws == nil {
			return nil, fmt.Errorf("aws engine not available")
		}
		rtID, _ := cfg.Parameters["route_table_id"].(string)
		cidr, _ := cfg.Parameters["destination_cidr"].(string)
		return r.aws.BlackholeRoute(ctx, rtID, cidr, cfg.Safety.DryRun)

	default:
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownChaosType, cfg.ChaosType)
	}
}

func (r *Runner) persistResult(ctx context.Context, experimentID string, result *domain.ExperimentResult) {
	if r.queries == nil {
		return
	}

	configJSON, _ := json.Marshal(result.Config)
	steadyJSON, _ := json.Marshal(result.SteadyState)
	injJSON, _ := json.Marshal(result.InjectionResult)
	obsJSON, _ := json.Marshal(result.Observations)
	rbJSON, _ := json.Marshal(result.RollbackResult)
	aiJSON, _ := json.Marshal(result.AIInsights)

	var completedAt pgtype.Timestamptz
	if result.CompletedAt != nil {
		completedAt = pgtype.Timestamptz{Time: *result.CompletedAt, Valid: true}
	}

	// Create or update the experiment record
	_, err := r.queries.CreateExperiment(ctx, db.CreateExperimentParams{
		ID:     experimentID,
		Config: configJSON,
		Status: string(result.Status),
		Phase:  string(result.Phase),
		StartedAt: pgtype.Timestamptz{
			Time:  *result.StartedAt,
			Valid: result.StartedAt != nil,
		},
	})
	if err != nil {
		// Already exists, update instead
		var hypothesis pgtype.Text
		if result.Hypothesis != nil {
			hypothesis = pgtype.Text{String: *result.Hypothesis, Valid: true}
		}
		var errText pgtype.Text
		if result.Error != nil {
			errText = pgtype.Text{String: *result.Error, Valid: true}
		}

		r.queries.UpdateExperiment(ctx, db.UpdateExperimentParams{
			ID:              experimentID,
			Status:          string(result.Status),
			Phase:           string(result.Phase),
			CompletedAt:     completedAt,
			SteadyState:     steadyJSON,
			Hypothesis:      hypothesis,
			InjectionResult: injJSON,
			Observations:    obsJSON,
			RollbackResult:  rbJSON,
			Error:           errText,
			AiInsights:      aiJSON,
		})
	}
}

func extractStringSlice(params map[string]any, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}
