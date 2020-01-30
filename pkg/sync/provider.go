package sync

import (
	"context"
	"fmt"
)

const (
	// GitTagStateMode is a mode of state management where Flux uses a git tag for managing Flux state
	GitTagStateMode = "git"

	// NativeStateMode is a mode of state management where Flux uses native Kubernetes resources for managing Flux state
	NativeStateMode = "secret"
)

// VerifySignaturesMode represents the strategy to use when choosing which commits to GPG-verify between the flux sync tag and the tip of the flux branch
type VerifySignaturesMode string

const (
	// VerifySignaturesModeDefault - get the default behavior when casting
	VerifySignaturesModeDefault = ""

	// VerifySignaturesModeNone (default) - don't verify any commits
	VerifySignaturesModeNone = "none"

	// VerifySignaturesModeAll - consider all possible commits
	VerifySignaturesModeAll = "all"

	// VerifySignaturesModeFirstParent - consider only commits on the chain of
	// first parents (i.e. don't consider commits merged from another branch)
	VerifySignaturesModeFirstParent = "first-parent"
)

// ToVerifySignaturesMode converts a string to a VerifySignaturesMode
func ToVerifySignaturesMode(s string) (VerifySignaturesMode, error) {
	switch s {
	case VerifySignaturesModeDefault:
		return VerifySignaturesModeNone, nil
	case VerifySignaturesModeNone:
		return VerifySignaturesModeNone, nil
	case VerifySignaturesModeAll:
		return VerifySignaturesModeAll, nil
	case VerifySignaturesModeFirstParent:
		return VerifySignaturesModeFirstParent, nil
	default:
		return VerifySignaturesModeNone, fmt.Errorf("'%s' is not a valid git-verify-signatures-mode", s)
	}
}

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
