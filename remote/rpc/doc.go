package rpc

/*

This is a `net/rpc`-compatible implementation of a client and server
for `flux/remote.Platform`.

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

At the client end, we also need to transmission errors -- e.g., a
response timing out, or the connection closing abruptly. These are
treated as "Fatal" errors; that is, they should result in a
disconnection of the daemon as well as being returned to the caller.

*/
