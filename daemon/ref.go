package daemon

import (
	"context"
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

type Ref struct {
	sync.RWMutex
	platform remote.Platform
}

func NewRef(platform remote.Platform) (pr *Ref) {
	return &Ref{platform: platform}
}

func (pr *Ref) Platform() remote.Platform {
	pr.RLock()
	defer pr.RUnlock()
	return pr.platform
}

func (pr *Ref) UpdatePlatform(platform remote.Platform) {
	pr.Lock()
	pr.platform = platform
	pr.Unlock()
}

// remote.Platform implementation so clients don't need to be refactored around
// Platform() API

func (pr *Ref) Ping(ctx context.Context) error {
	return pr.Platform().Ping(ctx)
}

func (pr *Ref) Version(ctx context.Context) (string, error) {
	return pr.Platform().Version(ctx)
}

func (pr *Ref) Export(ctx context.Context) ([]byte, error) {
	return pr.Platform().Export(ctx)
}

func (pr *Ref) ListServices(ctx context.Context, namespace string) ([]flux.ServiceStatus, error) {
	return pr.Platform().ListServices(ctx, namespace)
}

func (pr *Ref) ListImages(ctx context.Context, spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	return pr.Platform().ListImages(ctx, spec)
}

func (pr *Ref) UpdateManifests(ctx context.Context, spec update.Spec) (job.ID, error) {
	return pr.Platform().UpdateManifests(ctx, spec)
}

func (pr *Ref) SyncNotify(ctx context.Context) error {
	return pr.Platform().SyncNotify(ctx)
}

func (pr *Ref) JobStatus(ctx context.Context, id job.ID) (job.Status, error) {
	return pr.Platform().JobStatus(ctx, id)
}

func (pr *Ref) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	return pr.Platform().SyncStatus(ctx, ref)
}

func (pr *Ref) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	return pr.Platform().GitRepoConfig(ctx, regenerate)
}
