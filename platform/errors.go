package platform

import (
	"github.com/weaveworks/flux"
)

func UnavailableError(err error) error {
	return flux.UserConfigProblem{flux.BaseError{
		Help: `Cannot contact fluxd

To service this request, we need to ask the agent running in your
cluster (fluxd) for some information. But we can't connect to it at
present.

This may be because it's not running at all, or because it has
temporarily disconnected. You can check whether fluxd is connected with

    fluxctl status

If you are sure fluxd is running, you can simply wait a few seconds
and try the operation again.

If you are not sure if fluxd is running, please consult the
installation instructions in our documentation:

    https://github.com/weaveworks/flux/blob/master/site/installing.md

If you are still stuck, please log an issue:

    https://github.com/weaveworks/flux/issues

`,
		Err: err,
	}}
}
