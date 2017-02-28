package http

import (
	"errors"

	"github.com/weaveworks/flux"
)

var ErrorDeprecated = &flux.BaseError{
	Help: `The API endpoint requested appears to have been deprecated.

This indicates your client (fluxctl) needs to be updated: please see

    https://github.com/weaveworks/flux/releases

If you still have this problem after upgrading, please file an issue at

    https://github.com/weaveworks/flux/issues

mentioning what you were attempting to do, and the output of

    fluxctl status
`,
	Err: errors.New("API endpoint deprecated"),
}

func MakeAPINotFound(path string) *flux.BaseError {
	return &flux.BaseError{
		Help: `The API endpoint requested is not supported by this server.

This indicates that your client (probably fluxctl) is either out of
date, or faulty. Please see

    https://github.com/weaveworks/flux/releases

for releases of fluxctl.

If you still have problems, please file an issue at

    https://github.com/weaveworks/flux/issues

mentioning what you were attempting to do, and the output of

    fluxctl status

and include this path:

    ` + path + `
`,
		Err: errors.New("API endpoint not found"),
	}
}
