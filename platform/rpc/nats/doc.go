package nats

/* A `MessageBus` implementation that uses NATS (https://nats.io/).

The responsibility of the MessageBus is to:

 1. Connect to the platform for an instance (hand out Platform
 implementations given an instance ID); and,
 2. Register a remote platform against an instance ID.

In NATS terms, this means:

 1. Supplying a platform implementation that will send requests to
 NATS, addressed to the instance, and relay the responses back; and,
 2. Listening for platform requests for an instance, relay them to the
 remote platform, and relay the responses back to NATS

*/
