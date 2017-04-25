package rpc

import (
	"net/rpc"

	"github.com/weaveworks/flux/platform"
)

// CategoriseError turns any old error from an RPC call into a
// FatalError or a (non-fatal) ClusterError; or nil, if it happens to
// be nil to start off with.
func CategoriseRPCError(err error) error {
	if err == nil {
		return nil
	}
	// An rpc.ServerError is what the net/rpc package returns if the
	// remote method call returned an error; anything else indicates a
	// problem with the PRC _mechanism_, which we regard as a
	// connection-terminating error.
	if _, ok := err.(rpc.ServerError); !ok {
		return platform.FatalError{err}
	}
	return platform.ClusterError(err)
}
