package remote

import (
	fluxerr "github.com/fluxcd/flux/pkg/errors"
)

func UnavailableError(err error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Help: `Cannot contact Flux daemon

To service this request, we need to ask the agent running in your
cluster (fluxd) for some information. But we can't connect to it at
present.

This may be because it's not running at all, is temporarily
disconnected or has been firewalled.

If you are sure Flux is running, you can simply wait a few seconds
and try the operation again.

Otherwise, please consult the installation instructions in our
documentation:

    https://docs.fluxcd.io/en/latest/get-started/

If you are still stuck, please log an issue:

    https://github.com/fluxcd/flux/issues

`,
		Err: err,
	}
}

func UpgradeNeededError(err error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Help: `Your Flux daemon needs to be upgraded

    ` + err.Error() + `

To service this request, we need to ask the agent running in your
cluster (fluxd) to perform an operation on our behalf, but the
version you have running is too old to understand the request.

Please install the latest version of Flux and try again.

`,
		Err: err,
	}
}

func UnsupportedResourceKind(err error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Help: err.Error() + `

The version of the agent running in your cluster (fluxd) can release updates to
the following kinds of pod controller: Deployments, DaemonSets, StatefulSets
and CronJobs. When new kinds are added to Kubernetes, we try to support them as
quickly as possible - check here to see if a new version of Flux is available:

	https://github.com/fluxcd/flux/releases

Releasing by Service is not supported - if you're using an old version of
fluxctl that accepts the '--service' argument you will need to get a new one
that matches your agent.
`,
		Err: err,
	}
}

func ClusterError(err error) error {
	return &fluxerr.Error{
		Type: fluxerr.User,
		Help: `Error from Flux daemon

The Flux daemon (fluxd) reported this error:

    ` + err.Error() + `

which indicates that it is running, but cannot complete the request.

Thus may be because the request wasn't valid; e.g., you asked for
something in a namespace that doesn't exist.

Otherwise, it is likely to be an ongoing problem until fluxd is
updated and/or redeployed. For help, please consult the installation
instructions:

    https://docs.fluxcd.io/en/latest/get-started/

If you are still stuck, please log an issue:

    https://github.com/fluxcd/flux/issues

`,
		Err: err,
	}
}

// Wrap errors in this to indicate that the server should be
// considered dead, and disconnected.
type FatalError struct {
	Err error
}

func (err FatalError) Error() string {
	return err.Err.Error()
}
