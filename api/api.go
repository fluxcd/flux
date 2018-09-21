package api

import "github.com/weaveworks/flux/api/v11"

// Server defines the minimal interface a Flux must satisfy to adequately serve a
// connecting fluxctl. This interface specifically does not facilitate connecting
// to Weave Cloud.
type Server interface {
	v11.Server
}

// UpstreamServer is the interface a Flux must satisfy in order to communicate with
// Weave Cloud.
type UpstreamServer interface {
	v11.Server
	v11.Upstream
}
