package release

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"

	"github.com/go-kit/kit/log"
)

var (
	oldImage          = "quay.io/weaveworks/helloworld:master-a000001"
	oldImageID, _     = flux.ParseImageID(oldImage)
	sidecarImage      = "quay.io/weaveworks/sidecar:master-a000002"
	sidecarImageID, _ = flux.ParseImageID(sidecarImage)
	hwSvcID, _        = flux.ParseServiceID("default/helloworld")
	hwSvcSpec, _      = flux.ParseServiceSpec(hwSvcID.String())
	hwSvc             = platform.Service{
		ID: hwSvcID,
		Containers: platform.ContainersOrExcuse{
			Containers: []platform.Container{
				platform.Container{
					Name:  "helloworld",
					Image: oldImage,
				},
				platform.Container{
					Name:  "sidecar",
					Image: "quay.io/weaveworks/sidecar:master-a000002",
				},
			},
		},
	}

	oldLockedImg     = "quay.io/weaveworks/locked-service:1"
	newLockedImg     = "quay.io/weaveworks/locked-service:2"
	newLockedID, _   = flux.ParseImageID(newLockedImg)
	lockedSvcID, _   = flux.ParseServiceID("default/locked-service")
	lockedSvcSpec, _ = flux.ParseServiceSpec(lockedSvcID.String())
	lockedSvc        = platform.Service{
		ID: lockedSvcID,
		Containers: platform.ContainersOrExcuse{
			Containers: []platform.Container{
				platform.Container{
					Name:  "locked-service",
					Image: oldLockedImg,
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
	testSvcSpec, _ = flux.ParseServiceSpec(testScv.ID.String())

	allSvcs = []platform.Service{
		hwSvc,
		lockedSvc,
		testScv,
	}
	newImageID, _ = flux.ParseImageID("quay.io/weaveworks/helloworld:master-a000002")
	timeNow       = time.Now()
	mockRegistry  = registry.NewMockRegistry([]flux.Image{
		flux.Image{
			ImageID:   newImageID,
			CreatedAt: &timeNow,
		},
		flux.Image{
			ImageID:   newLockedID,
			CreatedAt: &timeNow,
		},
	}, nil)
)

func setup(t *testing.T, mocks instance.Instance) (*Releaser, func()) {
	repo, cleanup := gittest.Repo(t)

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

func Test_FilterLogic(t *testing.T) {
	mockPlatform := &platform.MockPlatform{
		AllServicesAnswer: allSvcs,
		SomeServicesAnswer: []platform.Service{
			hwSvc,
			lockedSvc,
		},
	}

	mockConfig := &instance.MockConfigurer{
		Config: instance.Config{
			Services: map[flux.ServiceID]instance.ServiceConfig{
				lockedSvcID: {
					Automated: false,
					Locked:    true,
				},
			},
		},
		Error: nil,
	}

	for _, tst := range []struct {
		Name     string
		Spec     flux.ReleaseSpec
		Expected flux.ReleaseResult
	}{
		// ignored if: excluded OR not included OR not correct image.
		{
			Name: "not included",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{hwSvcSpec},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					PerContainer: []flux.ContainerUpdate{
						flux.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
			},
		}, {
			Name: "excluded",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAll},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{lockedSvcID},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					PerContainer: []flux.ContainerUpdate{
						flux.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  Excluded,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		}, {
			Name: "not image",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAll},
				ImageSpec:    flux.ImageSpecFromID(newImageID),
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					PerContainer: []flux.ContainerUpdate{
						flux.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  DifferentImage,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotInCluster,
				},
			},
		},
		// skipped if: not ignored AND (locked or not found in cluster)
		// else: service is pending.
		{
			Name: "skipped & service is pending",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{flux.ServiceSpecAll},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					PerContainer: []flux.ContainerUpdate{
						flux.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  Locked,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		},
		{
			Name: "all overrides spec",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{hwSvcSpec, flux.ServiceSpecAll},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSuccess,
					PerContainer: []flux.ContainerUpdate{
						flux.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  Locked,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		},
	} {
		releaser, cleanup := setup(t, instance.Instance{
			Platform: mockPlatform,
			Registry: mockRegistry,
			Config:   mockConfig,
		})
		defer cleanup()

		testRelease(t, releaser, tst.Name, tst.Spec, tst.Expected)
	}
}

func Test_ImageStatus(t *testing.T) {
	mockPlatform := &platform.MockPlatform{
		AllServicesAnswer: allSvcs,
		SomeServicesAnswer: []platform.Service{
			hwSvc,
			lockedSvc,
			testScv,
		},
	}

	mockConfig := &instance.MockConfigurer{
		Config: instance.Config{
			Services: map[flux.ServiceID]instance.ServiceConfig{
				lockedSvcID: {
					Automated: false,
					Locked:    true,
				},
			},
		},
		Error: nil,
	}

	upToDateRegistry := registry.NewMockRegistry([]flux.Image{
		flux.Image{
			ImageID:   oldImageID,
			CreatedAt: &timeNow,
		},
		flux.Image{
			ImageID:   sidecarImageID,
			CreatedAt: &timeNow,
		},
	}, nil)

	testSvcSpec, _ := flux.ParseServiceSpec(testScv.ID.String())
	for _, tst := range []struct {
		Name     string
		Spec     flux.ReleaseSpec
		Expected flux.ReleaseResult
	}{
		{
			Name: "image not found",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{testSvcSpec},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  ImageNotFound,
				},
			},
		}, {
			Name: "image up to date",
			Spec: flux.ReleaseSpec{
				ServiceSpecs: []flux.ServiceSpec{hwSvcSpec},
				ImageSpec:    flux.ImageSpecLatest,
				Kind:         flux.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: flux.ReleaseResult{
				flux.ServiceID("default/helloworld"): flux.ServiceResult{
					Status: flux.ReleaseStatusSkipped,
					Error:  ImageUpToDate,
				},
				flux.ServiceID("default/locked-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): flux.ServiceResult{
					Status: flux.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
			},
		},
	} {
		releaser, cleanup := setup(t, instance.Instance{
			Platform: mockPlatform,
			Config:   mockConfig,
			Registry: upToDateRegistry,
		})
		defer cleanup()

		testRelease(t, releaser, tst.Name, tst.Spec, tst.Expected)
	}
}

func testRelease(t *testing.T, releaser *Releaser, name string, spec flux.ReleaseSpec, expected flux.ReleaseResult) {
	results := flux.ReleaseResult{}
	moreJobs, err := releaser.release(flux.InstanceID("doesn't matter"),
		&jobs.Job{
			Params: jobs.ReleaseJobParams{
				ReleaseSpec: spec,
			},
		}, func(f string, a ...interface{}) {
			fmt.Printf(f+"\n", a...)
		}, func(r flux.ReleaseResult) {
			if r == nil {
				t.Errorf("%s - result update called with nil value", name)
			}
			results = r
		})
	if err != nil {
		t.Error(err)
	}
	if len(moreJobs) > 0 {
		t.Errorf("%s - did not expect followup jobs, got %#v", name, moreJobs)
	}
	println()
	PrintResults(os.Stdout, results, true)
	println()
	if !reflect.DeepEqual(expected, results) {
		t.Errorf("%s - expected:\n%#v, got:\n%#v", name, expected, results)
	}
}

type mockEventWriter struct {
	events []history.Event
}

func (ew *mockEventWriter) LogEvent(e history.Event) error {
	if ew.events == nil {
		ew.events = make([]history.Event, 0)
	}
	ew.events = append(ew.events, e)
	return nil
}

func Test_LogEvent(t *testing.T) {
	mockEventWriter := mockEventWriter{}
	inst := instance.Instance{
		EventWriter: &mockEventWriter,
	}
	release := flux.Release{
		ID:        "testID",
		CreatedAt: time.Now(),
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
		Done:      true,
		Priority:  100,
		Status:    flux.ReleaseStatusSuccess,
		Log:       []string{"Step 1", "Step 2"},
		Cause: flux.ReleaseCause{
			User:    "TestUser",
			Message: "Test message",
		},
		Spec: flux.ReleaseSpec{
			ServiceSpecs: []flux.ServiceSpec{hwSvcSpec, lockedSvcSpec},
			ImageSpec:    flux.ImageSpecFromID(newImageID),
			Kind:         flux.ReleaseKindExecute,
			Excludes:     []flux.ServiceID{},
		},
		Result: flux.ReleaseResult{
			flux.ServiceID("default/helloworld"): flux.ServiceResult{
				Status: flux.ReleaseStatusSuccess,
				PerContainer: []flux.ContainerUpdate{
					flux.ContainerUpdate{
						Container: "helloworld",
						Current:   oldImageID,
						Target:    newImageID,
					},
				},
			},
			flux.ServiceID("default/locked-service"): flux.ServiceResult{
				Status: flux.ReleaseStatusIgnored,
				Error:  DifferentImage,
			},
			flux.ServiceID("default/test-service"): flux.ServiceResult{
				Status: flux.ReleaseStatusIgnored,
				Error:  DifferentImage,
			},
		},
	}
	err := logEvent(&inst, nil, release)
	if err != nil {
		t.Fatal(err)
	}
	if len(mockEventWriter.events) != 1 {
		t.Fatalf("Expecting single event but got %v", len(mockEventWriter.events))
	}
	event1 := mockEventWriter.events[0]
	if len(event1.ServiceIDs) != 1 {
		t.Fatalf("Expecting single service to be reported as altered but got %v services", len(event1.ServiceIDs))
	}
}
