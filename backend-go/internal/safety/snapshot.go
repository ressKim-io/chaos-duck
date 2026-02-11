package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxSnapshots = 1000

// SnapshotManager captures and stores state snapshots before chaos injection
type SnapshotManager struct {
	mu        sync.RWMutex
	snapshots map[string]map[string]any
	queries   *db.Queries
}

// NewSnapshotManager creates a new SnapshotManager
func NewSnapshotManager(queries *db.Queries) *SnapshotManager {
	return &SnapshotManager{
		snapshots: make(map[string]map[string]any),
		queries:   queries,
	}
}

// CaptureK8sSnapshot captures Kubernetes resource state before mutation.
// The actual K8s API calls are delegated to the engine layer;
// this method stores the provided state data.
func (sm *SnapshotManager) CaptureK8sSnapshot(
	ctx context.Context,
	experimentID string,
	namespace string,
	state map[string]any,
) (map[string]any, error) {
	snapshot := map[string]any{
		"type":        "k8s",
		"namespace":   namespace,
		"captured_at": time.Now().UTC().Format(time.RFC3339),
		"resources":   state,
	}

	sm.mu.Lock()
	sm.evictIfNeeded()
	sm.snapshots[experimentID] = snapshot
	sm.mu.Unlock()

	sm.persistSnapshot(ctx, experimentID, snapshot)
	return snapshot, nil
}

// CaptureAWSSnapshot captures AWS resource state before mutation
func (sm *SnapshotManager) CaptureAWSSnapshot(
	ctx context.Context,
	experimentID string,
	resourceType string,
	resourceID string,
	state map[string]any,
) (map[string]any, error) {
	snapshot := map[string]any{
		"type":          "aws",
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"captured_at":   time.Now().UTC().Format(time.RFC3339),
		"state":         state,
	}

	sm.mu.Lock()
	sm.evictIfNeeded()
	sm.snapshots[experimentID] = snapshot
	sm.mu.Unlock()

	sm.persistSnapshot(ctx, experimentID, snapshot)
	return snapshot, nil
}

// evictIfNeeded removes the oldest snapshot when at capacity.
// Must be called with sm.mu held.
func (sm *SnapshotManager) evictIfNeeded() {
	if len(sm.snapshots) < maxSnapshots {
		return
	}
	// Evict the first key found (pseudo-random from map iteration)
	for k := range sm.snapshots {
		delete(sm.snapshots, k)
		break
	}
}

// GetSnapshot returns the stored snapshot for an experiment
func (sm *SnapshotManager) GetSnapshot(experimentID string) (map[string]any, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	snap, ok := sm.snapshots[experimentID]
	return snap, ok
}

// DeleteSnapshot removes the snapshot for an experiment
func (sm *SnapshotManager) DeleteSnapshot(experimentID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.snapshots, experimentID)
}

// ListSnapshots returns all stored snapshots
func (sm *SnapshotManager) ListSnapshots() map[string]map[string]any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]map[string]any, len(sm.snapshots))
	for k, v := range sm.snapshots {
		result[k] = v
	}
	return result
}

// RestoreFromSnapshot compares the stored snapshot with current state
// and returns a list of detected drifts. currentState should be fetched
// by the caller from the appropriate engine (K8s/AWS).
func (sm *SnapshotManager) RestoreFromSnapshot(
	experimentID string,
	currentState map[string]any,
) (map[string]any, error) {
	snapshot, ok := sm.GetSnapshot(experimentID)
	if !ok {
		return nil, fmt.Errorf("no snapshot found for experiment %s", experimentID)
	}

	restored := map[string]any{
		"experiment_id": experimentID,
		"actions":       []map[string]any{},
	}

	snapshotType, _ := snapshot["type"].(string)
	switch snapshotType {
	case "k8s":
		actions := sm.restoreK8s(snapshot, currentState)
		restored["actions"] = actions
	case "aws":
		actions := sm.restoreAws(snapshot, currentState)
		restored["actions"] = actions
	}

	return restored, nil
}

// restoreK8s detects drift between snapshot and current K8s state.
// Checks for missing pods that existed in the snapshot.
func (sm *SnapshotManager) restoreK8s(snapshot, currentState map[string]any) []map[string]any {
	actions := []map[string]any{}

	resources, _ := snapshot["resources"].(map[string]any)
	if resources == nil {
		return actions
	}

	// Get snapshot pod names
	snapshotPods, _ := resources["pods"].([]any)
	snapshotPodNames := make(map[string]bool)
	for _, p := range snapshotPods {
		if pod, ok := p.(map[string]any); ok {
			if name, ok := pod["name"].(string); ok {
				snapshotPodNames[name] = true
			}
		}
	}

	if len(snapshotPodNames) == 0 {
		return actions
	}

	// Get current pod names
	currentPods, _ := currentState["pods"].([]any)
	currentPodNames := make(map[string]bool)
	for _, p := range currentPods {
		if pod, ok := p.(map[string]any); ok {
			if name, ok := pod["name"].(string); ok {
				currentPodNames[name] = true
			}
		}
	}

	// Detect missing pods
	namespace, _ := snapshot["namespace"].(string)
	for podName := range snapshotPodNames {
		if !currentPodNames[podName] {
			log.Printf("Pod %s was in snapshot but is now missing in %s", podName, namespace)
			actions = append(actions, map[string]any{
				"action": "pod_missing",
				"name":   podName,
				"status": "detected",
			})
		}
	}

	return actions
}

// restoreAws detects drift between snapshot and current AWS state.
// Checks for EC2 instance state changes.
func (sm *SnapshotManager) restoreAws(snapshot, currentState map[string]any) []map[string]any {
	actions := []map[string]any{}

	state, _ := snapshot["state"].(map[string]any)
	if state == nil {
		return actions
	}

	resourceType, _ := snapshot["resource_type"].(string)
	if resourceType == "ec2" {
		snapshotState, _ := state["state"].(string)
		instanceID, _ := state["instance_id"].(string)
		currentInstanceState, _ := currentState["state"].(string)

		if instanceID != "" && snapshotState != "" && currentInstanceState != "" && currentInstanceState != snapshotState {
			actions = append(actions, map[string]any{
				"action":         "state_drift",
				"instance_id":    instanceID,
				"snapshot_state": snapshotState,
				"current_state":  currentInstanceState,
			})
		}
	}

	return actions
}

func (sm *SnapshotManager) persistSnapshot(ctx context.Context, experimentID string, snapshot map[string]any) {
	if sm.queries == nil {
		return
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Failed to marshal snapshot: %v", err)
		return
	}

	snapshotType, _ := snapshot["type"].(string)
	ns, _ := snapshot["namespace"].(string)

	_, err = sm.queries.CreateSnapshot(ctx, db.CreateSnapshotParams{
		ExperimentID: experimentID,
		Type:         snapshotType,
		Namespace:    pgtype.Text{String: ns, Valid: ns != ""},
		Data:         data,
		CapturedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		log.Printf("DB persistence skipped for snapshot: %v", err)
	}
}
