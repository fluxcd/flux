// This package defines the types for Flux API version 9.
package v9

import (
	"context"

	"github.com/fluxcd/flux/pkg/api/v6"
)

type Server interface {
	v6.NotDeprecated
}

type Upstream interface {
	v6.Upstream

	// ChangeNotify tells the daemon that we've noticed a change in
	// e.g., the git repo, or image registry, and now would be a good
	// time to update its state.
	NotifyChange(context.Context, Change) error
}
