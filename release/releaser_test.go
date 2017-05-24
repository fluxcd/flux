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
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/update"
)

var (
	oldImage          = "quay.io/weaveworks/helloworld:master-a000001"
	oldImageID, _     = flux.ParseImageID(oldImage)
	sidecarImage      = "quay.io/weaveworks/sidecar:master-a000002"
	sidecarImageID, _ = flux.ParseImageID(sidecarImage)
	hwSvcID, _        = flux.ParseServiceID("default/helloworld")
	hwSvcSpec, _      = update.ParseServiceSpec(hwSvcID.String())
	hwSvc             = cluster.Service{
		ID: hwSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				cluster.Container{
					Name:  "helloworld",
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
	newLockedID, _   = flux.ParseImageID(newLockedImg)
	lockedSvcID, _   = flux.ParseServiceID("default/locked-service")
	lockedSvcSpec, _ = update.ParseServiceSpec(lockedSvcID.String())
	lockedSvc        = cluster.Service{
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

	testScv = cluster.Service{
		ID: "default/test-service",
		Containers: cluster.ContainersOrExcuse{
			Containers: []cluster.Container{
				cluster.Container{
					Name:  "test-service",
					Image: "quay.io/weaveworks/test-service:1",
				},
			},
		},
	}
	testSvcSpec, _ = update.ParseServiceSpec(testScv.ID.String())

	allSvcs = []cluster.Service{
		hwSvc,
		lockedSvc,
		testScv,
	}
	newImageID, _ = flux.ParseImageID("quay.io/weaveworks/helloworld:master-a000002")
	timeNow       = time.Now()
	mockRegistry  = registry.NewMockRegistry([]flux.Image{
		flux.Image{
			ID:        newImageID,
			CreatedAt: timeNow,
		},
		flux.Image{
			ID:        newLockedID,
			CreatedAt: timeNow,
		},
	}, nil)
	mockManifests = &kubernetes.Manifests{}
)

func setup(t *testing.T) (*git.Checkout, func()) {
	return gittest.Checkout(t)
}

func Test_FilterLogic(t *testing.T) {
	mockCluster := &cluster.Mock{
		AllServicesFunc: func(string) ([]cluster.Service, error) {
			return allSvcs, nil
		},
		SomeServicesFunc: func([]flux.ServiceID) ([]cluster.Service, error) {
			return []cluster.Service{
				hwSvc,
				lockedSvc,
			}, nil
		},
	}

	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		// ignored if: excluded OR not included OR not correct image.
		{
			Name: "not included",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
			},
		}, {
			Name: "excluded",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{update.ServiceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{lockedSvcID},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  Excluded,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		}, {
			Name: "not image",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{update.ServiceSpecAll},
				ImageSpec:    update.ImageSpecFromID(newImageID),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  DifferentImage,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotInCluster,
				},
			},
		},
		// skipped if: not ignored AND (locked or not found in cluster)
		// else: service is pending.
		{
			Name: "skipped & service is pending",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{update.ServiceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  Locked,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		},
		{
			Name: "all overrides spec",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{hwSvcSpec, update.ServiceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: "helloworld",
							Current:   oldImageID,
							Target:    newImageID,
						},
					},
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  Locked,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  NotInCluster,
				},
			},
		},
	} {
		checkout, cleanup := setup(t)
		defer cleanup()
		testRelease(t, tst.Name, &ReleaseContext{
			Cluster:   mockCluster,
			Manifests: mockManifests,
			Registry:  mockRegistry,
			Repo:      checkout,
		}, tst.Spec, tst.Expected)
	}
}

func Test_ImageStatus(t *testing.T) {
	mockCluster := &cluster.Mock{
		AllServicesFunc: func(string) ([]cluster.Service, error) {
			return allSvcs, nil
		},
		SomeServicesFunc: func([]flux.ServiceID) ([]cluster.Service, error) {
			return []cluster.Service{
				hwSvc,
				lockedSvc,
				testScv,
			}, nil
		},
	}

	upToDateRegistry := registry.NewMockRegistry([]flux.Image{
		flux.Image{
			ID:        oldImageID,
			CreatedAt: timeNow,
		},
		flux.Image{
			ID:        sidecarImageID,
			CreatedAt: timeNow,
		},
	}, nil)

	testSvcSpec, _ := update.ParseServiceSpec(testScv.ID.String())
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		{
			Name: "image not found",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{testSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  ImageNotFound,
				},
			},
		}, {
			Name: "image up to date",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ServiceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ServiceID{},
			},
			Expected: update.Result{
				flux.ServiceID("default/helloworld"): update.ServiceResult{
					Status: update.ReleaseStatusSkipped,
					Error:  ImageUpToDate,
				},
				flux.ServiceID("default/locked-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
				flux.ServiceID("default/test-service"): update.ServiceResult{
					Status: update.ReleaseStatusIgnored,
					Error:  NotIncluded,
				},
			},
		},
	} {
		checkout, cleanup := setup(t)
		defer cleanup()
		ctx := &ReleaseContext{
			Cluster:   mockCluster,
			Manifests: mockManifests,
			Repo:      checkout,
			Registry:  upToDateRegistry,
		}
		testRelease(t, tst.Name, ctx, tst.Spec, tst.Expected)
	}
}

func testRelease(t *testing.T, name string, ctx *ReleaseContext, spec update.ReleaseSpec, expected update.Result) {
	_, results, err := Release(ctx, spec, log.NewNopLogger())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, results) {
		t.Errorf("%s - expected:\n%#v, got:\n%#v", name, expected, results)
	}
}
