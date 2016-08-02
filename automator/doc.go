// Package automator implements continuous deployment. The automator sits on top
// of the platform and the registry.
//
//    +------------------------+
//    | automator              |
//    +------------------------+
//      ^             ^
//      | services    | images
//      |             |
//    +----------+  +----------+
//    | platform |  | registry |
//    +----------+  +----------+
//
// The automator is modeled as a Kubernetes-style reconciliation loop. It
// receives services from the platform. To start, services are received by
// polling; eventually, with some kind of subscription endpoint.
//
// Certain services are marked so that they will be continuously released by the
// automator. To start, this list of services is set by fluxctl and stored
// in-memory in fluxd; eventually, state may move somewhere else.
//
// Those services are modeled as state machines inside of the automator.
//
// State machine
//
//                       +---------+
//                  .----| Waiting |<-----------------------.
//                  |    +---------+                        |
//                  |         ^                             |
//                  |         |                             |
//                  |  Don't need release                   |
//                  |         |                             |
//    +---------+   v   +------------+                 +-----------+
//    | Deleted |------>| Refreshing |--Need release-->| Releasing |
//    +---------+       +------------+                 +-----------+
//         ^                  |
//         |          No longer candidate
//         |                  |
//         '------------------'
//
// When a service is first discovered on the platform, it enters the Refreshing
// state. Refreshing means reading the current image from the platform, and
// polling the image registry for all available images.
//
// A service is in the Refreshing state very briefly; only long enough to decide
// the next state. If the service is no longer a candidate for automation, it is
// deleted. If the most recent registry image is newer than the currently active
// platform image, the service needs reconciliation and enters the Releasing
// state. Otherwise, the service is stable and enters the Waiting state.
//
// In the Releasing state, the automator fetches the config repo, manipulates
// the relevant resource definition files, performs relevant platform.Releases,
// and commits and pushes the files. It may fail at any step. No matter the
// outcome, the managed service moves to the Waiting state.
//
// In the Waiting state, the service is idle for a given timeout. When the
// timeout expires, the service moves to the Refreshing state. In the future,
// services may stay in the Waiting state indefinitely, until kicked out by a
// hook from e.g. CircleCI.
package automator
