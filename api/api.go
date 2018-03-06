package api

import (
	"github.com/weaveworks/flux/api/v9"
)

// Server defines the minimal interface a Flux must satisfy to adequately serve a
// connecting fluxctl. This interface specifically does not facilitate connecting
// to Weave Cloud.
type Server interface {
	v9.Server
}

// UpstreamServer is the interface a Flux must satisfy in order to communicate with
// Weave Cloud.
type UpstreamServer interface {
	v9.Server
	v9.Upstream
}
