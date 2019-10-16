package http

import (
	"errors"

	fluxerr "github.com/fluxcd/flux/pkg/errors"
)

var ErrorDeprecated = &fluxerr.Error{
	Type: fluxerr.Missing,
	Help: `The API endpoint requested appears to have been deprecated.

This indicates your client (fluxctl) needs to be updated: please see

    https://github.com/fluxcd/flux/releases

If you still have this problem after upgrading, please file an issue at

    https://github.com/fluxcd/flux/issues

mentioning what you were attempting to do.
`,
	Err: errors.New("API endpoint deprecated"),
}

var ErrorUnauthorized = &fluxerr.Error{
	Type: fluxerr.User,
	Help: `The request failed authentication

This most likely means you have a missing or incorrect token. Please
make sure you supply a service token, either by setting the
environment variable FLUX_SERVICE_TOKEN, or using the argument --token
with fluxctl.

`,
	Err: errors.New("request failed authentication"),
}

func MakeAPINotFound(path string) *fluxerr.Error {
	return &fluxerr.Error{
		Type: fluxerr.Missing,
		Help: `The API endpoint requested is not supported by this server.

This indicates that your client (probably fluxctl) is either out of
date, or faulty. Please see

    https://github.com/fluxcd/flux/releases

for releases of fluxctl.

If you still have problems, please file an issue at

    https://github.com/fluxcd/flux/issues

mentioning what you were attempting to do, and include this path:

    ` + path + `
`,
		Err: errors.New("API endpoint not found"),
	}
}
