package safety

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/chaosduck/backend-go/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

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
	sm.snapshots[experimentID] = snapshot
	sm.mu.Unlock()

	sm.persistSnapshot(ctx, experimentID, snapshot)
	return snapshot, nil
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
