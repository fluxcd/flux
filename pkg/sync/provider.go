package sync

import (
	"context"
)

const (
	// GitTagStateMode is a mode of state management where Flux uses a git tag for managing Flux state
	GitTagStateMode = "git"

	// NativeStateMode is a mode of state management where Flux uses native Kubernetes resources for managing Flux state
	NativeStateMode = "secret"
)

type State interface {
	// GetRevision fetches the recorded revision, returning an empty
	// string if none has been recorded yet.
	GetRevision(ctx context.Context) (string, error)
	// UpdateMarker records the high water mark
	UpdateMarker(ctx context.Context, revision string) error
	// DeleteMarker removes the high water mark
	DeleteMarker(ctx context.Context) error
	// String returns a string representation of where the state is
	// recorded (e.g., for referring to it in logs)
	String() string
}
