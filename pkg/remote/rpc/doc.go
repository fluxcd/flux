/*

This is a `net/rpc`-compatible implementation of a client and server
for `flux/api.Server`.

The purpose is to be able to access a daemon from an upstream
service. The daemon makes an outbound connection (over, say,
websockets), then the service can make RPC calls over that connection.

On errors:

Errors from the daemon can come in two varieties: application errors
(i.e., a `*(flux/errors).Error`), and internal errors (any other
`error`). We need to transmit these faithfully over `net/rpc`, which
only accounts for `error` (and flattens them to strings for
transmission).

To send application errors, we construct response values that are
effectively a union of the actual response type, and the error type.

At the client end, we also need to deal with transmission errors --
e.g., a response timing out, or the connection closing abruptly. These
are treated as "Fatal" errors; that is, they should result in a
disconnection of the daemon as well as being returned to the caller.

On versions:

The RPC protocol is versioned, because server code (in the daemon) is
deployed independently of client code (in the upstream service).

We share the RPC protocol versions with the API, because the endpoint
for connecting to an upstream service (`/api/flux/<version>/register`)
is part of the API.

Since one client (upstream service) has connections to many servers
(daemons), it's the client that has explicit versions in the code. The
server code always implements just the most recent version.

For backwards-incompatible changes, we must bump the protocol version
(and create a new `RegisterDaemon` endpoint).

On contexts:

Sadly, `net/rpc` does not support context.Context, and never will. So
we must ignore the contexts passed in. If we change the RPC mechanism,
we may be able to address this.

*/
package rpc
