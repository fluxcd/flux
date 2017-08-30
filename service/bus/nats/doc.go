package nats

/* A `MessageBus` implementation that uses NATS (https://nats.io/).

The responsibility of the MessageBus is to:

 1. Act as a client to the platform for an instance (hand out Platform
 implementations given an instance ID); and,

 2. Register a remote platform against an instance ID, and thereafter
 convey requests to it and responses back to the requester.

In NATS terms, this means:

 1. Supplying a platform implementation that will send requests to
 NATS, addressed to the instance, and relay the response back; and,

 2. Listening for platform requests for an instance, relay them to the
 remote platform, and relay the responses back to NATS.

```
                               send     +---+   reply Q
                               reply    |   |   sub
                            +----------->   +----------+
                            |           | B |          |
+--------+         +--------+-------+   |   |   +------v-------+
| daemon <----wss--> Subscribe loop |   | U |   | natsPlatform |
+--------+         +--------+-------+   |   |   +------+-------+
                            ^           | S |          |
                            |           |   |          |
                            +-----------+   <----------+
                               wildcard |   |   send
                               sub      +---+   request
```

On errors:

The encoding of responses is a bit different to that used by
`remote/rpc`, simply because we have to do all the work. Usually,
`net/rpc` would put an error return value in its envelope. Since we're
not using `net/rpc` here (it doesn't seem to map well onto NATS
messages), we have to include those in our message payload, which
already includes the application-level error if any.

Of the latter, there will be internal errors from the daemon, internal
errors from the service, and transmission errors (which we
recapitulate as application errors saying "the daemon is
unavailable").

*/
