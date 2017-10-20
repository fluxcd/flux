package remote

import (
	"context"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

// For historical reasons, the (versioned) interface is called
// `Platform`.

type PlatformV4 interface {
	Ping(context.Context) error
	Version(context.Context) (string, error)
	// Deprecated
	//	AllServices(maybeNamespace string, ignored flux.ServiceIDSet) ([]Service, error)
	//	SomeServices([]flux.ServiceID) ([]Service, error)
	//	Apply([]ServiceDefinition) error
}

type PlatformV5 interface {
	PlatformV4
	// We still support this, for bootstrapping; but it might
	// reasonably be moved to the daemon interface, or removed in
	// favour of letting people use their cluster-specific tooling.
	Export(context.Context) ([]byte, error)
	// Deprecated
	//	Sync(SyncDef) error
}

// In which we move functionality that refers to the Git repo or image
// registry into the platform. Methods that we no longer use are
// deprecated, so this does not include the previous definitions,
// though it does include some their methods.
type PlatformV6 interface {
	PlatformV5
	// These are new, or newly moved to this interface
	ListServices(ctx context.Context, namespace string) ([]flux.ControllerStatus, error)
	ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error)
	// Send a spec for updating config to the daemon
	UpdateManifests(context.Context, update.Spec) (job.ID, error)
	// Poke the daemon to sync with git
	SyncNotify(context.Context) error
	// Ask the daemon where it's up to with syncing
	SyncStatus(ctx context.Context, ref string) ([]string, error)
	// Ask the daemon where it's up to with job processing
	JobStatus(context.Context, job.ID) (job.Status, error)
	// Get the daemon's public SSH key
	GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error)
}

// Platform is the SPI for the daemon; i.e., it's all the things we
// have to ask to the daemon, rather than the service.
type Platform interface {
	PlatformV6
}

// Wrap errors in this to indicate that the platform should be
// considered dead, and disconnected.
type FatalError struct {
	Err error
}

func (err FatalError) Error() string {
	return err.Err.Error()
}
