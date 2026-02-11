package safety

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotManagerCaptureK8s(t *testing.T) {
	sm := NewSnapshotManager(nil) // no DB

	state := map[string]any{
		"pods":        []any{},
		"services":    []any{},
		"deployments": []any{},
	}

	snap, err := sm.CaptureK8sSnapshot(context.Background(), "exp-1", "default", state)
	require.NoError(t, err)

	assert.Equal(t, "k8s", snap["type"])
	assert.Equal(t, "default", snap["namespace"])
	assert.NotEmpty(t, snap["captured_at"])
	assert.NotNil(t, snap["resources"])

	// Verify retrieval
	retrieved, ok := sm.GetSnapshot("exp-1")
	assert.True(t, ok)
	assert.Equal(t, snap, retrieved)
}

func TestSnapshotManagerCaptureAWS(t *testing.T) {
	sm := NewSnapshotManager(nil)

	state := map[string]any{
		"instance_id": "i-12345",
		"state":       "running",
	}

	snap, err := sm.CaptureAWSSnapshot(context.Background(), "exp-2", "ec2", "i-12345", state)
	require.NoError(t, err)

	assert.Equal(t, "aws", snap["type"])
	assert.Equal(t, "ec2", snap["resource_type"])
	assert.Equal(t, "i-12345", snap["resource_id"])

	retrieved, ok := sm.GetSnapshot("exp-2")
	assert.True(t, ok)
	assert.Equal(t, snap, retrieved)
}

func TestSnapshotManagerDelete(t *testing.T) {
	sm := NewSnapshotManager(nil)

	sm.CaptureK8sSnapshot(context.Background(), "exp-1", "default", map[string]any{})

	_, ok := sm.GetSnapshot("exp-1")
	assert.True(t, ok)

	sm.DeleteSnapshot("exp-1")

	_, ok = sm.GetSnapshot("exp-1")
	assert.False(t, ok)

	// Deleting nonexistent should not panic
	sm.DeleteSnapshot("nonexistent")
}

func TestSnapshotManagerGetNonexistent(t *testing.T) {
	sm := NewSnapshotManager(nil)

	_, ok := sm.GetSnapshot("nope")
	assert.False(t, ok)
}

func TestSnapshotManagerListSnapshots(t *testing.T) {
	sm := NewSnapshotManager(nil)

	sm.CaptureK8sSnapshot(context.Background(), "exp-1", "default", map[string]any{})
	sm.CaptureAWSSnapshot(context.Background(), "exp-2", "ec2", "i-123", map[string]any{})

	list := sm.ListSnapshots()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "exp-1")
	assert.Contains(t, list, "exp-2")
}

func TestSnapshotManagerListSnapshotsEmpty(t *testing.T) {
	sm := NewSnapshotManager(nil)
	list := sm.ListSnapshots()
	assert.Empty(t, list)
}

func TestRestoreFromSnapshotK8sMissingPods(t *testing.T) {
	sm := NewSnapshotManager(nil)

	// Snapshot with 3 pods
	state := map[string]any{
		"pods": []any{
			map[string]any{"name": "web-1", "namespace": "default"},
			map[string]any{"name": "web-2", "namespace": "default"},
			map[string]any{"name": "web-3", "namespace": "default"},
		},
	}
	sm.CaptureK8sSnapshot(context.Background(), "exp-1", "default", state)

	// Current state: only web-1 remains
	currentState := map[string]any{
		"pods": []any{
			map[string]any{"name": "web-1", "namespace": "default"},
		},
	}

	result, err := sm.RestoreFromSnapshot("exp-1", currentState)
	require.NoError(t, err)

	actions, _ := result["actions"].([]map[string]any)
	assert.Len(t, actions, 2, "should detect 2 missing pods")

	// Collect missing pod names
	missingNames := make([]string, 0, len(actions))
	for _, a := range actions {
		assert.Equal(t, "pod_missing", a["action"])
		assert.Equal(t, "detected", a["status"])
		missingNames = append(missingNames, a["name"].(string))
	}
	assert.Contains(t, missingNames, "web-2")
	assert.Contains(t, missingNames, "web-3")
}

func TestRestoreFromSnapshotK8sNoDrift(t *testing.T) {
	sm := NewSnapshotManager(nil)

	state := map[string]any{
		"pods": []any{
			map[string]any{"name": "web-1"},
		},
	}
	sm.CaptureK8sSnapshot(context.Background(), "exp-1", "default", state)

	currentState := map[string]any{
		"pods": []any{
			map[string]any{"name": "web-1"},
		},
	}

	result, err := sm.RestoreFromSnapshot("exp-1", currentState)
	require.NoError(t, err)

	actions, _ := result["actions"].([]map[string]any)
	assert.Empty(t, actions, "no drift should be detected")
}

func TestRestoreFromSnapshotAWSDrift(t *testing.T) {
	sm := NewSnapshotManager(nil)

	state := map[string]any{
		"instance_id": "i-12345",
		"state":       "running",
	}
	sm.CaptureAWSSnapshot(context.Background(), "exp-2", "ec2", "i-12345", state)

	// EC2 instance was stopped
	currentState := map[string]any{
		"instance_id": "i-12345",
		"state":       "stopped",
	}

	result, err := sm.RestoreFromSnapshot("exp-2", currentState)
	require.NoError(t, err)

	actions, _ := result["actions"].([]map[string]any)
	require.Len(t, actions, 1)
	assert.Equal(t, "state_drift", actions[0]["action"])
	assert.Equal(t, "running", actions[0]["snapshot_state"])
	assert.Equal(t, "stopped", actions[0]["current_state"])
}

func TestRestoreFromSnapshotAWSNoDrift(t *testing.T) {
	sm := NewSnapshotManager(nil)

	state := map[string]any{
		"instance_id": "i-12345",
		"state":       "running",
	}
	sm.CaptureAWSSnapshot(context.Background(), "exp-2", "ec2", "i-12345", state)

	currentState := map[string]any{
		"instance_id": "i-12345",
		"state":       "running",
	}

	result, err := sm.RestoreFromSnapshot("exp-2", currentState)
	require.NoError(t, err)

	actions, _ := result["actions"].([]map[string]any)
	assert.Empty(t, actions)
}

func TestRestoreFromSnapshotNotFound(t *testing.T) {
	sm := NewSnapshotManager(nil)

	_, err := sm.RestoreFromSnapshot("nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no snapshot found")
}
