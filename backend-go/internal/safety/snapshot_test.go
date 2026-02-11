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
