package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractStringSliceFromStringSlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []string{"i-111", "i-222", "i-333"},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{"i-111", "i-222", "i-333"}, result)
}

func TestExtractStringSliceFromAnySlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []any{"i-aaa", "i-bbb"},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{"i-aaa", "i-bbb"}, result)
}

func TestExtractStringSliceFromAnySliceMixed(t *testing.T) {
	params := map[string]any{
		"items": []any{"valid", 42, "also-valid"},
	}
	result := extractStringSlice(params, "items")
	// Non-string items should be skipped
	assert.Equal(t, []string{"valid", "also-valid"}, result)
}

func TestExtractStringSliceMissingKey(t *testing.T) {
	params := map[string]any{
		"other": "value",
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Nil(t, result)
}

func TestExtractStringSliceWrongType(t *testing.T) {
	params := map[string]any{
		"instance_ids": "not-a-slice",
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Nil(t, result)
}

func TestExtractStringSliceEmptySlice(t *testing.T) {
	params := map[string]any{
		"instance_ids": []string{},
	}
	result := extractStringSlice(params, "instance_ids")
	assert.Equal(t, []string{}, result)
}

func TestExtractStringSliceNilMap(t *testing.T) {
	result := extractStringSlice(nil, "key")
	assert.Nil(t, result)
}
