package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	assert.True(t, errors.Is(ErrEmergencyStop, ErrEmergencyStop))
	assert.True(t, errors.Is(ErrBlastRadiusExceeded, ErrBlastRadiusExceeded))
	assert.True(t, errors.Is(ErrExperimentNotFound, ErrExperimentNotFound))
	assert.True(t, errors.Is(ErrTimeout, ErrTimeout))
	assert.True(t, errors.Is(ErrNamespaceConfirmation, ErrNamespaceConfirmation))
	assert.True(t, errors.Is(ErrUnknownChaosType, ErrUnknownChaosType))
	assert.True(t, errors.Is(ErrAIServiceUnavailable, ErrAIServiceUnavailable))

	// Ensure errors are distinct
	assert.False(t, errors.Is(ErrEmergencyStop, ErrTimeout))
	assert.False(t, errors.Is(ErrBlastRadiusExceeded, ErrExperimentNotFound))
}

func TestErrorMessages(t *testing.T) {
	assert.Equal(t, "emergency stop is active", ErrEmergencyStop.Error())
	assert.Equal(t, "blast radius exceeded", ErrBlastRadiusExceeded.Error())
	assert.Equal(t, "experiment not found", ErrExperimentNotFound.Error())
	assert.Equal(t, "operation timed out", ErrTimeout.Error())
}
