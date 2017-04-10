package release

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"

	"github.com/go-kit/kit/log"
)

var (
	hwSvcID, _ = flux.ParseServiceID("default/helloworld")
	hwSvc      = platform.Service{
		ID: hwSvcID,
		Containers: platform.ContainersOrExcuse{
			Containers: []platform.Container{
				platform.Container{
					Name:  "helloworld",
					Image: "quay.io/weaveworks/helloworld:master-a000001",
				},
				platform.Container{
					Name:  "sidecar",
					Image: "quay.io/weaveworks/sidecar:master-a000002",
				},
			},
		},
	}
	lockedSvcID, _ = flux.ParseServiceID("default/locked-service")
	lockedSvc      = platform.Service{
		ID: lockedSvcID,
		Containers: platform.ContainersOrExcuse{
			Containers: []platform.Container{
				platform.Container{
					Name:  "locked-service",
					Image: "quay.io/weaveworks/locked-service:1",
				},
			},
		},
	}
	testScv = platform.Service{
		ID: "default/test-service",
		Containers: platform.ContainersOrExcuse{
			Containers: []platform.Container{
				platform.Container{
					Name:  "test-service",
					Image: "quay.io/weaveworks/test-service:1",
				},
			},
		},
	}
	allSvcs = []platform.Service{
		hwSvc,
		lockedSvc,
		testScv,
	}
	imageID, _   = flux.ParseImageID("quay.io/weaveworks/helloworld:master-a000002")
	timeNow      = time.Now()
	mockRegistry = registry.NewMockRegistry([]flux.Image{
		flux.Image{
			ImageID:   imageID,
			CreatedAt: &timeNow,
		},
	}, nil)
)

func setup(t *testing.T, mocks instance.Instance) (*Releaser, func()) {
	repo, cleanup := setupRepo(t)

	if mocks.Platform == nil {
		mocks.Platform = &platform.MockPlatform{}
	}
	if mocks.Registry == nil {
		mocks.Registry = registry.NewMockRegistry(nil, nil)
	}
	if mocks.Config == nil {
		config := instance.Config{}
		mocks.Config = &instance.MockConfigurer{config, nil}
	}
	mocks.Repo = repo
	events := history.NewMock()
	mocks.EventReader, mocks.EventWriter = events, events
	mocks.Logger = log.NewNopLogger()

	instancer := &instance.MockInstancer{&mocks, nil}
	return NewReleaser(instancer), cleanup
}

func TestMissingFromPlatform(t *testing.T) {
	releaser, cleanup := setup(t, instance.Instance{})
	defer cleanup()

	output := func(f string, a ...interface{}) {
		fmt.Printf(f+"\n", a...)
	}

	spec := jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ServiceSpec: flux.ServiceSpecAll,
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindPlan,
		},
	}

	results := flux.ReleaseResult{}
	update := func(r flux.ReleaseResult) {
		if r == nil {
			t.Errorf("result update called with nil value")
		}
		results = r
	}

	moreJobs, err := releaser.release(flux.InstanceID("unimportant"),
		&jobs.Job{Params: spec}, output, update)
	if err != nil {
		t.Error(err)
	}
	if len(moreJobs) > 0 {
		t.Errorf("did not expect follow-on jobs, got %v", moreJobs)
	}

	expected := flux.ReleaseResult{
		flux.ServiceID("default/helloworld"): flux.ServiceResult{
			Status: flux.ReleaseStatusIgnored,
			Error:  "not in running system",
		},
		flux.ServiceID("default/locked-service"): flux.ServiceResult{
			Status: flux.ReleaseStatusIgnored,
			Error:  "not in running system",
		},
		flux.ServiceID("default/test-service"): flux.ServiceResult{
			Status: flux.ReleaseStatusIgnored,
			Error:  "not in running system",
		},
	}
	if !reflect.DeepEqual(expected, results) {
		t.Errorf("expected %#v, got %#v", expected, results)
	}

	spec = jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ServiceSpec: flux.ServiceSpec("default/helloworld"),
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindPlan,
		},
	}
	results = flux.ReleaseResult{}
	moreJobs, err = releaser.release(flux.InstanceID("unimportant"),
		&jobs.Job{Params: spec}, output, update)
	if err != nil {
		t.Error(err)
	}
	if len(moreJobs) > 0 {
		t.Errorf("did not expect followup jobs, got %#v", moreJobs)
	}

	expected = flux.ReleaseResult{
		flux.ServiceID("default/helloworld"): flux.ServiceResult{
			Status: flux.ReleaseStatusSkipped,
			Error:  "not in running system",
		},
	}
	if !reflect.DeepEqual(expected, results) {
		t.Errorf("expected %#v, got %#v", expected, results)
	}
}

func TestUpdateOne(t *testing.T) {
	serviceID, _ := flux.ParseServiceID("default/helloworld")
	mockPlatform := &platform.MockPlatform{
		SomeServicesArgTest: func(req []flux.ServiceID) error {
			if len(req) != 1 || req[0] != serviceID {
				return errors.New("expected exactly {default/helloworld}")
			}
			return nil
		},
		SomeServicesAnswer: []platform.Service{
			hwSvc,
		},
	}

	releaser, cleanup := setup(t, instance.Instance{
		Platform: mockPlatform,
		Registry: mockRegistry,
	})
	defer cleanup()

	spec := jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ServiceSpec: flux.ServiceSpec("default/helloworld"),
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindExecute,
		},
	}

	results := flux.ReleaseResult{}
	moreJobs, err := releaser.release(flux.InstanceID("instance 3"),
		&jobs.Job{Params: spec}, func(f string, a ...interface{}) {
			fmt.Printf(f+"\n", a...)
		}, func(r flux.ReleaseResult) {
			if r == nil {
				t.Errorf("result update called with nil value")
			}
			results = r
		})
	if err != nil {
		t.Error(err)
	}

	println()
	PrintResults(os.Stdout, results, true)
	println()

	if len(moreJobs) > 0 {
		t.Errorf("did not expect follow-on jobs, got %v", moreJobs)
	}
	if len(results) != 1 {
		t.Errorf("expected one service in results, got %v", results)
	}
	result, ok := results[serviceID]
	if !ok {
		t.Errorf("expected entry for %s but there was none", serviceID)
	}
	if result.Status != flux.ReleaseStatusSuccess {
		t.Errorf("expected entry to be success, but was %s", result.Status)
	}
}

func Test_492_OnUpdateImage_OnlyShowServicePertainingToImage(t *testing.T) {
	lockedID, _ := flux.ParseServiceID("default/locked-service")

	mockConfig := &instance.MockConfigurer{
		Config: instance.Config{
			Services: map[flux.ServiceID]instance.ServiceConfig{
				lockedID: {
					Automated: false,
					Locked:    true,
				},
			},
		},
		Error: nil,
	}

	mockPlatform := &platform.MockPlatform{
		AllServicesAnswer: allSvcs,
		SomeServicesAnswer: []platform.Service{
			hwSvc,
			testScv,
		},
	}

	releaser, cleanup := setup(t, instance.Instance{
		Platform: mockPlatform,
		Registry: mockRegistry,
		Config:   mockConfig,
	})
	defer cleanup()

	spec := jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ServiceSpec: flux.ServiceSpecAll,
			ImageSpec:   flux.ImageSpecFromID(imageID),
			Kind:        flux.ReleaseKindExecute,
		},
	}

	results := flux.ReleaseResult{}
	moreJobs, err := releaser.release(flux.InstanceID("instance 3"),
		&jobs.Job{Params: spec}, func(f string, a ...interface{}) {
			fmt.Printf(f+"\n", a...)
		}, func(r flux.ReleaseResult) {
			if r == nil {
				t.Errorf("result update called with nil value")
			}
			results = r
		})
	if err != nil {
		t.Error(err)
	}

	println()
	PrintResults(os.Stdout, results, true)
	println()

	if len(moreJobs) > 0 {
		t.Errorf("did not expect follow-on jobs, got %v", moreJobs)
	}
	if len(results) != 1 {
		t.Errorf("expected one service in results, got %v", len(results))
	}
	result, ok := results[hwSvcID]
	if !ok {
		t.Errorf("expected entry for %s but there was none", hwSvcID)
	}
	if result.Status != flux.ReleaseStatusSuccess {
		t.Errorf("expected entry to be success, but was %s", result.Status)
	}
}

func Test_492_OnUpdateService_OnlyShowServicePertainingToService(t *testing.T) {
	mockPlatform := &platform.MockPlatform{
		SomeServicesArgTest: func(req []flux.ServiceID) error {
			//if len(req) != 1 || req[0] != hwSvcID {
			//	return errors.New("expected exactly {default/helloworld}")
			//}
			return nil
		},
		AllServicesAnswer: allSvcs,
		SomeServicesAnswer: []platform.Service{
			hwSvc,
		},
	}

	releaser, cleanup := setup(t, instance.Instance{
		Platform: mockPlatform,
		Registry: mockRegistry,
	})
	defer cleanup()

	svc, _ := flux.ParseServiceSpec(hwSvcID.String())
	spec := jobs.ReleaseJobParams{
		ReleaseSpec: flux.ReleaseSpec{
			ServiceSpec: svc,
			ImageSpec:   flux.ImageSpecLatest,
			Kind:        flux.ReleaseKindExecute,
		},
	}

	results := flux.ReleaseResult{}
	moreJobs, err := releaser.release(flux.InstanceID("instance 3"),
		&jobs.Job{Params: spec}, func(f string, a ...interface{}) {
			fmt.Printf(f+"\n", a...)
		}, func(r flux.ReleaseResult) {
			if r == nil {
				t.Errorf("result update called with nil value")
			}
			results = r
		})
	if err != nil {
		t.Error(err)
	}

	println()
	PrintResults(os.Stdout, results, true)
	println()

	if len(moreJobs) > 0 {
		t.Errorf("did not expect follow-on jobs, got %v", moreJobs)
	}
	if len(results) != 1 {
		t.Errorf("expected one service in results, got %v", len(results))
	}
	result, ok := results[hwSvcID]
	if !ok {
		t.Errorf("expected entry for %s but there was none", hwSvcID)
	}
	if result.Status != flux.ReleaseStatusSuccess {
		t.Errorf("expected entry to be success, but was %s", result.Status)
	}
}
