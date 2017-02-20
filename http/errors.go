package http

import (
	"errors"

	"github.com/weaveworks/flux"
)

var ErrorGone = flux.BaseError{
	Help: `The API endpoint requested appears to have been deprecated.

This indicates your client (fluxctl) needs to be updated: please see

    https://github.com/weaveworks/flux/releases

If you still have this problem after upgrading, please file an issue at

    https://github.com/weaveworks/flux/issues

mentioning what you were attempting to do, and the output of

    fluxctl status
`,
	Err: errors.New("API endpoint missing"),
}
