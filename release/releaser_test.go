package release

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/image"
	registryMock "github.com/weaveworks/flux/registry/mock"
	"github.com/weaveworks/flux/update"
)

var (
	// This must match the value in cluster/kubernetes/testfiles/data.go
	container = "goodbyeworld"

	oldImage      = "quay.io/weaveworks/helloworld:master-a000001"
	oldRef, _     = image.ParseRef(oldImage)
	sidecarImage  = "quay.io/weaveworks/sidecar:master-a000002"
	sidecarRef, _ = image.ParseRef(sidecarImage)
	hwSvcID, _    = flux.ParseResourceID("default:deployment/helloworld")
	hwSvcSpec, _  = update.ParseResourceSpec(hwSvcID.String())
	hwSvc         = cluster.Controller{
		ID: hwSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				cluster.Container{
					Name:  container,
					Image: oldImage,
				},
				cluster.Container{
					Name:  "sidecar",
					Image: "quay.io/weaveworks/sidecar:master-a000002",
				},
			},
		},
	}

	oldLockedImg     = "quay.io/weaveworks/locked-service:1"
	newLockedImg     = "quay.io/weaveworks/locked-service:2"
	newLockedID, _   = image.ParseRef(newLockedImg)
	lockedSvcID, _   = flux.ParseResourceID("default:deployment/locked-service")
	lockedSvcSpec, _ = update.ParseResourceSpec(lockedSvcID.String())
	lockedSvc        = cluster.Controller{
		ID: lockedSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				cluster.Container{
					Name:  "locked-service",
					Image: oldLockedImg,
				},
			},
		},
	}

	testSvc = cluster.Controller{
		ID: flux.MustParseResourceID("default:deployment/test-service"),
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				cluster.Container{
					Name:  "test-service",
					Image: "quay.io/weaveworks/test-service:1",
				},
			},
		},
	}
	testSvcSpec, _ = update.ParseResourceSpec(testSvc.ID.String())

	allSvcs = []cluster.Controller{
		hwSvc,
		lockedSvc,
		testSvc,
	}
	newRef, _    = image.ParseRef("quay.io/weaveworks/helloworld:master-a000002")
	timeNow      = time.Now()
	mockRegistry = &registryMock.Registry{
		Images: []image.Info{
			{
				ID:        newRef,
				CreatedAt: timeNow,
			},
			{
				ID:        newLockedID,
				CreatedAt: timeNow,
			},
		},
	}
	mockManifests = &kubernetes.Manifests{}
)

func setup(t *testing.T) (*git.Checkout, func()) {
	return gittest.Checkout(t)
}

func Test_FilterLogic(t *testing.T) {
	mockCluster := &cluster.Mock{
		AllServicesFunc: func(string) ([]cluster.Controller, error) {
			return allSvcs, nil
		},
		SomeServicesFunc: func([]flux.ResourceID) ([]cluster.Controller, error) {
			return []cluster.Controller{
				hwSvc,
				lockedSvc,
			}, nil
		},
	}

	notInRepoService := "default:deployment/notInRepo"
	notInRepoSpec, _ := update.ParseResourceSpec(notInRepoService)
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		// ignored if: excluded OR not included OR not correct image.
		{
			Name: "not included",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: container,
							Current:   oldRef,
							Target:    newRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
			},
		}, {
			Name: "excluded",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{lockedSvcID},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: container,
							Current:   oldRef,
							Target:    newRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.Excluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.NotInCluster,
				},
			},
		}, {
			Name: "not image",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: container,
							Current:   oldRef,
							Target:    newRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.DifferentImage,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotInCluster,
				},
			},
		},
		// skipped if: not ignored AND (locked or not found in cluster)
		// else: service is pending.
		{
			Name: "skipped & service is pending",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: container,
							Current:   oldRef,
							Target:    newRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.Locked,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.NotInCluster,
				},
			},
		},
		{
			Name: "all overrides spec",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec, update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: container,
							Current:   oldRef,
							Target:    newRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.Locked,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.NotInCluster,
				},
			},
		},
		{
			Name: "service not in repo",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{notInRepoSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID(notInRepoService): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.NotInRepo,
				},
			},
		},
	} {
		checkout, cleanup := setup(t)
		defer cleanup()
		testRelease(t, tst.Name, &ReleaseContext{
			cluster:   mockCluster,
			manifests: mockManifests,
			registry:  mockRegistry,
			repo:      checkout,
		}, tst.Spec, tst.Expected)
	}
}

func Test_ImageStatus(t *testing.T) {
	mockCluster := &cluster.Mock{
		AllServicesFunc: func(string) ([]cluster.Controller, error) {
			return allSvcs, nil
		},
		SomeServicesFunc: func([]flux.ResourceID) ([]cluster.Controller, error) {
			return []cluster.Controller{
				hwSvc,
				lockedSvc,
				testSvc,
			}, nil
		},
	}

	upToDateRegistry := &registryMock.Registry{
		Images: []image.Info{
			{
				ID:        oldRef,
				CreatedAt: timeNow,
			},
			{
				ID:        sidecarRef,
				CreatedAt: timeNow,
			},
		},
	}

	testSvcSpec, _ := update.ParseResourceSpec(testSvc.ID.String())
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		{
			Name: "image not found",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{testSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.DoesNotUseImage,
				},
			},
		}, {
			Name: "image up to date",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.ImageUpToDate,
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.NotIncluded,
				},
			},
		},
	} {
		checkout, cleanup := setup(t)
		defer cleanup()
		ctx := &ReleaseContext{
			cluster:   mockCluster,
			manifests: mockManifests,
			repo:      checkout,
			registry:  upToDateRegistry,
		}
		testRelease(t, tst.Name, ctx, tst.Spec, tst.Expected)
	}
}

func testRelease(t *testing.T, name string, ctx *ReleaseContext, spec update.ReleaseSpec, expected update.Result) {
	results, err := Release(ctx, spec, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, results) {
		t.Errorf("%s - expected:\n%#v, got:\n%#v", name, expected, results)
	}
}
