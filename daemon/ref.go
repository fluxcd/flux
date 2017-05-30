package daemon

import (
	"sync"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/ssh"
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

func (pr *Ref) Ping() error {
	return pr.Platform().Ping()
}

func (pr *Ref) Version() (string, error) {
	return pr.Platform().Version()
}

func (pr *Ref) Export() ([]byte, error) {
	return pr.Platform().Export()
}

func (pr *Ref) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	return pr.Platform().ListServices(namespace)
}

func (pr *Ref) ListImages(spec update.ServiceSpec) ([]flux.ImageStatus, error) {
	return pr.Platform().ListImages(spec)
}

func (pr *Ref) UpdateManifests(spec update.Spec) (job.ID, error) {
	return pr.Platform().UpdateManifests(spec)
}

func (pr *Ref) SyncNotify() error {
	return pr.Platform().SyncNotify()
}

func (pr *Ref) JobStatus(id job.ID) (job.Status, error) {
	return pr.Platform().JobStatus(id)
}

func (pr *Ref) SyncStatus(ref string) ([]string, error) {
	return pr.Platform().SyncStatus(ref)
}

func (pr *Ref) PublicSSHKey(regenerate bool) (ssh.PublicKey, error) {
	return pr.Platform().PublicSSHKey(regenerate)
}
