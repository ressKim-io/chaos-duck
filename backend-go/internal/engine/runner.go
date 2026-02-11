package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/probe"
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
	aiClient    *http.Client
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
		aiClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Run executes the full 5-phase experiment lifecycle with timeout enforcement
func (r *Runner) Run(ctx context.Context, experimentID string, cfg domain.ExperimentConfig) (*domain.ExperimentResult, error) {
	if err := r.esm.CheckEmergencyStop(); err != nil {
		return nil, err
	}

	// Enforce timeout on the entire experiment lifecycle
	timeoutSec := cfg.Safety.TimeoutSeconds
	if timeoutSec < 1 {
		timeoutSec = 30
	}
	if timeoutSec > 120 {
		timeoutSec = 120
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

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

	// Build probes from config
	probes := r.buildProbes(cfg)
	var probeResults []map[string]any

	// Phase 1: Steady State
	if cfg.TargetNamespace != nil && r.k8s != nil {
		steadyState, err := r.k8s.GetSteadyState(ctx, *cfg.TargetNamespace)
		if err != nil {
			log.Printf("Steady state capture failed: %v", err)
		} else {
			result.SteadyState = steadyState
			r.snapshotMgr.CaptureK8sSnapshot(ctx, experimentID, *cfg.TargetNamespace, steadyState)
		}
	}

	// Execute SOT (Start of Test) probes
	for _, p := range probes {
		if p.Mode() == domain.ProbeModeSOT {
			pr := probe.SafeExecute(ctx, p)
			probeResults = append(probeResults, map[string]any{
				"probe": pr.ProbeName, "type": pr.ProbeType, "passed": pr.Passed,
			})
			if !pr.Passed {
				log.Printf("SOT probe %s failed, aborting experiment", pr.ProbeName)
				result.Status = domain.StatusFailed
				errStr := fmt.Sprintf("SOT probe %s failed", pr.ProbeName)
				result.Error = &errStr
				r.persistResult(ctx, experimentID, result)
				return result, fmt.Errorf("%s", errStr)
			}
		}
	}

	// AI: review steady state
	if cfg.AIEnabled && result.SteadyState != nil {
		if review, err := r.callAI("/review-steady-state", map[string]any{
			"steady_state": result.SteadyState,
		}); err == nil {
			aiInsights["steady_state_review"] = review
		} else {
			log.Printf("AI steady state review failed: %v", err)
		}
	}

	// Phase 2: Hypothesis
	result.Phase = domain.PhaseHypothesis
	if cfg.AIEnabled {
		body := map[string]any{
			"topology":   result.SteadyState,
			"target":     cfg.Name,
			"chaos_type": string(cfg.ChaosType),
		}
		if resp, err := r.callAI("/hypotheses", body); err == nil {
			if h, ok := resp["hypothesis"].(string); ok {
				result.Hypothesis = &h
			}
		} else {
			log.Printf("AI hypothesis generation failed: %v", err)
		}
	}

	// Safety: require confirmation for production namespaces
	if cfg.TargetNamespace != nil {
		if err := safety.RequireConfirmation(*cfg.TargetNamespace, "prod*", cfg.Safety.RequireConfirmation); err != nil {
			result.Status = domain.StatusFailed
			errStr := err.Error()
			result.Error = &errStr
			r.persistResult(ctx, experimentID, result)
			return result, err
		}
	}

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

	// Execute ON_CHAOS probes
	for _, p := range probes {
		if p.Mode() == domain.ProbeModeOnChaos {
			pr := probe.SafeExecute(ctx, p)
			probeResults = append(probeResults, map[string]any{
				"probe": pr.ProbeName, "type": pr.ProbeType, "passed": pr.Passed,
			})
		}
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

	// AI: compare observations with steady state
	if cfg.AIEnabled && result.Observations != nil {
		body := map[string]any{
			"steady_state": result.SteadyState,
			"observations": result.Observations,
			"hypothesis":   result.Hypothesis,
		}
		if analysis, err := r.callAI("/compare-observations", body); err == nil {
			aiInsights["observation_analysis"] = analysis
		} else {
			log.Printf("AI observation analysis failed: %v", err)
		}
	}

	// Execute EOT (End of Test) probes
	for _, p := range probes {
		if p.Mode() == domain.ProbeModeEOT {
			pr := probe.SafeExecute(ctx, p)
			probeResults = append(probeResults, map[string]any{
				"probe": pr.ProbeName, "type": pr.ProbeType, "passed": pr.Passed,
			})
		}
	}

	// Phase 5: Rollback
	result.Phase = domain.PhaseRollback
	result.Status = domain.StatusCompleted
	completedAt := time.Now().UTC()
	result.CompletedAt = &completedAt

	// AI: verify recovery
	if cfg.AIEnabled && result.SteadyState != nil && cfg.TargetNamespace != nil && r.k8s != nil {
		postState, err := r.k8s.GetSteadyState(ctx, *cfg.TargetNamespace)
		if err == nil {
			body := map[string]any{
				"original_state": result.SteadyState,
				"current_state":  postState,
			}
			if recovery, err := r.callAI("/verify-recovery", body); err == nil {
				aiInsights["recovery_verification"] = recovery
			} else {
				log.Printf("AI recovery verification failed: %v", err)
			}
		}
	}

	if len(aiInsights) > 0 {
		result.AIInsights = aiInsights
	}
	if len(probeResults) > 0 {
		if result.Observations == nil {
			result.Observations = make(map[string]any)
		}
		result.Observations["probe_results"] = probeResults
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
		if latencyMs < 1 || latencyMs > 60000 {
			return nil, fmt.Errorf("latency_ms must be 1-60000, got %d", latencyMs)
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
		if lossPercent < 1 || lossPercent > 100 {
			return nil, fmt.Errorf("loss_percent must be 1-100, got %d", lossPercent)
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
		if cores < 1 || cores > 64 {
			return nil, fmt.Errorf("cores must be 1-64, got %d", cores)
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

		if err := r.queries.UpdateExperiment(ctx, db.UpdateExperimentParams{
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
		}); err != nil {
			log.Printf("Failed to update experiment %s: %v", experimentID, err)
		}
	}
}

// callAI sends a JSON POST to the AI microservice and returns the response.
// Returns nil, error if the AI service is unavailable or returns an error.
func (r *Runner) callAI(path string, body any) (map[string]any, error) {
	if r.aiBaseURL == "" {
		return nil, fmt.Errorf("AI service URL not configured")
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	resp, err := r.aiClient.Post(
		r.aiBaseURL+path,
		"application/json",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse AI response: %w", err)
	}

	return result, nil
}

// buildProbes creates probe instances from experiment config
func (r *Runner) buildProbes(cfg domain.ExperimentConfig) []probe.Probe {
	var probes []probe.Probe
	for _, pc := range cfg.Probes {
		var p probe.Probe
		switch pc.Type {
		case domain.ProbeTypeHTTP:
			url, _ := pc.Properties["url"].(string)
			method, _ := pc.Properties["method"].(string)
			status := 200
			if v, ok := pc.Properties["expected_status"].(float64); ok {
				status = int(v)
			}
			bodyPattern, _ := pc.Properties["body_pattern"].(string)
			hp, err := probe.NewHTTPProbe(probe.HTTPProbeConfig{
				Name: pc.Name, Mode: pc.Mode, URL: url, Method: method,
				ExpectedStatus: status, BodyPattern: bodyPattern,
			})
			if err != nil {
				log.Printf("Failed to create HTTP probe %s: %v", pc.Name, err)
				continue
			}
			p = hp
		case domain.ProbeTypeCmd:
			command, _ := pc.Properties["command"].(string)
			exitCode := 0
			if v, ok := pc.Properties["expected_exit_code"].(float64); ok {
				exitCode = int(v)
			}
			p = probe.NewCmdProbe(probe.CmdProbeConfig{
				Name: pc.Name, Mode: pc.Mode, Command: command, ExpectedExitCode: exitCode,
			})
		case domain.ProbeTypeK8s:
			if r.k8s == nil {
				log.Printf("Skipping K8s probe %s: no K8s engine", pc.Name)
				continue
			}
			ns, _ := pc.Properties["namespace"].(string)
			kind, _ := pc.Properties["resource_kind"].(string)
			name, _ := pc.Properties["resource_name"].(string)
			p = probe.NewK8sProbe(probe.K8sProbeConfig{
				Name: pc.Name, Mode: pc.Mode, Clientset: r.k8s.Clientset(),
				Namespace: ns, ResourceKind: kind, ResourceName: name,
			})
		case domain.ProbeTypePrometheus:
			endpoint, _ := pc.Properties["endpoint"].(string)
			query, _ := pc.Properties["query"].(string)
			comparator, _ := pc.Properties["comparator"].(string)
			threshold := 0.0
			if v, ok := pc.Properties["threshold"].(float64); ok {
				threshold = v
			}
			p = probe.NewPromProbe(probe.PromProbeConfig{
				Name: pc.Name, Mode: pc.Mode, Endpoint: endpoint,
				Query: query, Comparator: comparator, Threshold: threshold,
			})
		default:
			log.Printf("Unknown probe type: %s", pc.Type)
			continue
		}
		probes = append(probes, p)
	}
	return probes
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
