package rpc

import (
	"io"

	"github.com/weaveworks/flux/platform"
)

// RPCClient is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV5 struct {
	*RPCClientV4
}

var _ platform.PlatformV5 = &RPCClientV5{}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV5(conn io.ReadWriteCloser) *RPCClientV5 {
	return &RPCClientV5{NewClientV4(conn)}
}

// Additional/overridden methods go here
