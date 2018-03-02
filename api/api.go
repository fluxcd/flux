package api

import (
	"context"

	"github.com/weaveworks/flux/api/v6"
)

type ServerV9 interface {
	v6.NotDeprecated
}

type UpstreamV9 interface {
	v6.Upstream

	// ChangeNotify tells the daemon that we've noticed a change in
	// e.g., the git repo, or image registry, and now would be a good
	// time to update its state.
	NotifyChange(context.Context, Change) error
}

// Server defines the minimal interface a Flux must satisfy to adequately serve a
// connecting fluxctl. This interface specifically does not facilitate connecting
// to Weave Cloud.
type Server interface {
	ServerV9
}

// UpstreamServer is the interface a Flux must satisfy in order to communicate with
// Weave Cloud.
type UpstreamServer interface {
	Server
	UpstreamV9
}
