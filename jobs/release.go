package jobs

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/release"
)

type releaseHandler struct {
	releaser *release.Releaser
}

func newReleaseHandler(instancer instance.Instancer, metrics release.Metrics) *releaseHandler {
	return &releaseHandler{
		releaser: release.NewReleaser(instancer, metrics),
	}
}

func (r *releaseHandler) Handle(j *flux.Job, q flux.JobWritePopper) error {
	return r.releaser.Release(j, j.Params.(flux.ReleaseJobParams), q)
}
