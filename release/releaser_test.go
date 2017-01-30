package release

import (
	"fmt"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"

	"github.com/go-kit/kit/log"
	metrics "github.com/go-kit/kit/metrics/discard"
)

func setup(t *testing.T) (*Releaser, func()) {
	repo, cleanup := setupRepo(t)

	config := instance.Config{}
	events := history.NewMock()

	inst := instance.New(
		&platform.MockPlatform{},
		registry.NewMockRegistry(nil, nil),
		&instance.MockConfigurer{config, nil},
		repo,
		log.NewNopLogger(),
		metrics.NewHistogram(),
		events,
		events,
	)

	instancer := &instance.MockInstancer{inst, nil}
	return NewReleaser(instancer, Metrics{}), cleanup
}

func TestNopCalculation(t *testing.T) {
	releaser, cleanup := setup(t)
	defer cleanup()

	spec := jobs.ReleaseJobParams{
		ServiceSpec: flux.ServiceSpecAll,
		ImageSpec:   flux.ImageSpecLatest,
	}

	var results flux.ReleaseResult
	moreJobs, err := releaser.release(flux.InstanceID("instance 3"),
		&jobs.Job{Params: spec}, func(f string, a ...interface{}) {
			fmt.Printf(f+"\n", a...)
		}, func(r flux.ReleaseResult) {
			results = r
		})
	if err != nil {
		t.Error(err)
	}
	if len(moreJobs) > 0 {
		t.Errorf("did not expect follow-on jobs, got %v", moreJobs)
	}
	if len(results) != 1 {
		t.Errorf("expected one service in results, got %v", results)
	}
}
