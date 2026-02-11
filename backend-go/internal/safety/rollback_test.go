package safety

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollbackManagerPushAndSize(t *testing.T) {
	rm := NewRollbackManager()

	assert.Equal(t, 0, rm.StackSize("exp-1"))

	rm.Push("exp-1", func() (map[string]any, error) {
		return nil, nil
	}, "action-1")

	assert.Equal(t, 1, rm.StackSize("exp-1"))

	rm.Push("exp-1", func() (map[string]any, error) {
		return nil, nil
	}, "action-2")

	assert.Equal(t, 2, rm.StackSize("exp-1"))
	assert.Equal(t, 0, rm.StackSize("exp-2"))
}

func TestRollbackManagerLIFOOrder(t *testing.T) {
	rm := NewRollbackManager()
	var order []string

	rm.Push("exp-1", func() (map[string]any, error) {
		order = append(order, "first")
		return map[string]any{"step": "first"}, nil
	}, "first")

	rm.Push("exp-1", func() (map[string]any, error) {
		order = append(order, "second")
		return map[string]any{"step": "second"}, nil
	}, "second")

	rm.Push("exp-1", func() (map[string]any, error) {
		order = append(order, "third")
		return map[string]any{"step": "third"}, nil
	}, "third")

	results := rm.Rollback("exp-1")

	require.Len(t, results, 3)
	// LIFO: third, second, first
	assert.Equal(t, []string{"third", "second", "first"}, order)
	assert.Equal(t, "third", results[0].Description)
	assert.Equal(t, "second", results[1].Description)
	assert.Equal(t, "first", results[2].Description)

	// Stack should be cleared
	assert.Equal(t, 0, rm.StackSize("exp-1"))
}

func TestRollbackManagerPartialFailure(t *testing.T) {
	rm := NewRollbackManager()

	rm.Push("exp-1", func() (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	}, "success-action")

	rm.Push("exp-1", func() (map[string]any, error) {
		return nil, assert.AnError
	}, "fail-action")

	results := rm.Rollback("exp-1")

	require.Len(t, results, 2)
	// fail-action runs first (LIFO)
	assert.Equal(t, "failed", results[0].Status)
	assert.Equal(t, "fail-action", results[0].Description)
	assert.NotEmpty(t, results[0].Error)

	assert.Equal(t, "success", results[1].Status)
	assert.Equal(t, "success-action", results[1].Description)
}

func TestRollbackManagerEmptyRollback(t *testing.T) {
	rm := NewRollbackManager()

	results := rm.Rollback("nonexistent")
	assert.Empty(t, results)
}

func TestRollbackManagerActiveExperiments(t *testing.T) {
	rm := NewRollbackManager()

	assert.Empty(t, rm.ActiveExperiments())

	rm.Push("exp-1", func() (map[string]any, error) { return nil, nil }, "a")
	rm.Push("exp-2", func() (map[string]any, error) { return nil, nil }, "b")

	active := rm.ActiveExperiments()
	assert.Len(t, active, 2)
	assert.Contains(t, active, "exp-1")
	assert.Contains(t, active, "exp-2")
}

func TestRollbackManagerRollbackAll(t *testing.T) {
	rm := NewRollbackManager()
	var count int

	rm.Push("exp-1", func() (map[string]any, error) {
		count++
		return nil, nil
	}, "a")
	rm.Push("exp-2", func() (map[string]any, error) {
		count++
		return nil, nil
	}, "b")
	rm.Push("exp-2", func() (map[string]any, error) {
		count++
		return nil, nil
	}, "c")

	all := rm.RollbackAll()

	assert.Equal(t, 3, count)
	assert.Len(t, all, 2)
	assert.Len(t, all["exp-1"], 1)
	assert.Len(t, all["exp-2"], 2)
	assert.Empty(t, rm.ActiveExperiments())
}
