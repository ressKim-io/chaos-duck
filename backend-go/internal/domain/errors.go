package domain

import "errors"

var (
	// ErrEmergencyStop is returned when the emergency stop is active
	ErrEmergencyStop = errors.New("emergency stop is active")

	// ErrBlastRadiusExceeded is returned when blast radius validation fails
	ErrBlastRadiusExceeded = errors.New("blast radius exceeded")

	// ErrExperimentNotFound is returned when an experiment ID is not found
	ErrExperimentNotFound = errors.New("experiment not found")

	// ErrTimeout is returned when an operation exceeds its timeout
	ErrTimeout = errors.New("operation timed out")

	// ErrNamespaceConfirmation is returned when production namespace requires confirmation
	ErrNamespaceConfirmation = errors.New("production namespace requires confirmation")

	// ErrUnknownChaosType is returned for unrecognised chaos types
	ErrUnknownChaosType = errors.New("unknown chaos type")

	// ErrAIServiceUnavailable is returned when the AI microservice is unreachable
	ErrAIServiceUnavailable = errors.New("AI service unavailable")
)
