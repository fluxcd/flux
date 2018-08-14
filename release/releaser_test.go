package release

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/cluster/kubernetes"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/git/gittest"
	"github.com/weaveworks/flux/image"
	registryMock "github.com/weaveworks/flux/registry/mock"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

var (
	// This must match the value in cluster/kubernetes/testfiles/data.go
	helloContainer   = "greeter"
	sidecarContainer = "sidecar"
	lockedContainer  = "locked-service"
	testContainer    = "test-service"

	oldImage      = "quay.io/weaveworks/helloworld:master-a000001"
	oldRef, _     = image.ParseRef(oldImage)
	sidecarImage  = "weaveworks/sidecar:master-a000001"
	sidecarRef, _ = image.ParseRef(sidecarImage)
	hwSvcID, _    = flux.ParseResourceID("default:deployment/helloworld")
	hwSvcSpec, _  = update.ParseResourceSpec(hwSvcID.String())
	hwSvc         = cluster.Controller{
		ID: hwSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  helloContainer,
					Image: oldRef,
				},
				{
					Name:  sidecarContainer,
					Image: sidecarRef,
				},
			},
		},
	}

	testServiceRef, _ = image.ParseRef("quay.io/weaveworks/test-service:1")

	oldLockedImg    = "quay.io/weaveworks/locked-service:1"
	oldLockedRef, _ = image.ParseRef(oldLockedImg)

	newLockedImg     = "quay.io/weaveworks/locked-service:2"
	newLockedRef, _  = image.ParseRef(newLockedImg)
	lockedSvcID, _   = flux.ParseResourceID("default:deployment/locked-service")
	lockedSvcSpec, _ = update.ParseResourceSpec(lockedSvcID.String())
	lockedSvc        = cluster.Controller{
		ID: lockedSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  lockedContainer,
					Image: oldLockedRef,
				},
			},
		},
	}

	semverHwImg    = "quay.io/weaveworks/helloworld:3.0.0"
	semverHwRef, _ = image.ParseRef(semverHwImg)
	semverSvcID    = flux.MustParseResourceID("default:deployment/semver")
	semverSvc      = cluster.Controller{
		ID: semverSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  helloContainer,
					Image: oldRef,
				},
			},
		},
	}
	semverSvcSpec, _ = update.ParseResourceSpec(semverSvc.ID.String())

	testSvcID = flux.MustParseResourceID("default:deployment/test-service")
	testSvc   = cluster.Controller{
		ID: testSvcID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  testContainer,
					Image: testServiceRef,
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
	newHwRef, _ = image.ParseRef("quay.io/weaveworks/helloworld:master-a000002")
	// this is what we expect things to be updated to
	newSidecarRef, _ = image.ParseRef("weaveworks/sidecar:master-a000002")
	// this is what we store in the registry cache
	canonSidecarRef, _ = image.ParseRef("index.docker.io/weaveworks/sidecar:master-a000002")

	timeNow  = time.Now()
	timePast = timeNow.Add(-1 * time.Minute)

	mockRegistry = &registryMock.Registry{
		Images: []image.Info{
			{
				ID:        semverHwRef,
				CreatedAt: timePast,
			},
			{
				ID:        newHwRef,
				CreatedAt: timeNow,
			},
			{
				ID:        newSidecarRef,
				CreatedAt: timeNow,
			},
			{
				ID:        newLockedRef,
				CreatedAt: timeNow,
			},
		},
	}
	mockManifests = &kubernetes.Manifests{}
)

func mockCluster(running ...cluster.Controller) *cluster.Mock {
	return &cluster.Mock{
		AllServicesFunc: func(string) ([]cluster.Controller, error) {
			return running, nil
		},
		SomeServicesFunc: func(ids []flux.ResourceID) ([]cluster.Controller, error) {
			var res []cluster.Controller
			for _, id := range ids {
				for _, svc := range running {
					if id == svc.ID {
						res = append(res, svc)
					}
				}
			}
			return res, nil
		},
	}
}

func setup(t *testing.T) (*git.Checkout, func()) {
	return gittest.Checkout(t)
}

var ignoredNotIncluded = update.ControllerResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotIncluded,
}

var ignoredNotInRepo = update.ControllerResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotInRepo,
}

var ignoredNotInCluster = update.ControllerResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotInCluster,
}

var skippedLocked = update.ControllerResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.Locked,
}

var skippedNotInCluster = update.ControllerResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.NotInCluster,
}

var skippedNotInRepo = update.ControllerResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.NotInRepo,
}

func Test_FilterLogic(t *testing.T) {
	cluster := mockCluster(hwSvc, lockedSvc) // no testsvc in cluster, but it _is_ in repo
	notInRepoService := "default:deployment/notInRepo"
	notInRepoSpec, _ := update.ParseResourceSpec(notInRepoService)
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		// ignored if: excluded OR not included OR not correct image.
		{
			Name: "include specific service",
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
							Container: helloContainer,
							Current:   oldRef,
							Target:    newHwRef,
						},
						update.ContainerUpdate{
							Container: sidecarContainer,
							Current:   sidecarRef,
							Target:    newSidecarRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
			},
		}, {
			Name: "exclude specific service",
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
							Container: helloContainer,
							Current:   oldRef,
							Target:    newHwRef,
						},
						update.ContainerUpdate{
							Container: sidecarContainer,
							Current:   sidecarRef,
							Target:    newSidecarRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.Excluded,
				},
				flux.MustParseResourceID("default:deployment/test-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):  skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):       skippedNotInCluster,
			},
		}, {
			Name: "update specific image",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newHwRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/helloworld"): update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{
						update.ContainerUpdate{
							Container: helloContainer,
							Current:   oldRef,
							Target:    newHwRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusIgnored,
					Error:  update.DifferentImage,
				},
				flux.MustParseResourceID("default:deployment/test-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):  skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):       skippedNotInCluster,
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
							Container: helloContainer,
							Current:   oldRef,
							Target:    newHwRef,
						},
						update.ContainerUpdate{
							Container: sidecarContainer,
							Current:   sidecarRef,
							Target:    newSidecarRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.Locked,
				},
				flux.MustParseResourceID("default:deployment/test-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):  skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):       skippedNotInCluster,
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
							Container: helloContainer,
							Current:   oldRef,
							Target:    newHwRef,
						},
						update.ContainerUpdate{
							Container: sidecarContainer,
							Current:   sidecarRef,
							Target:    newSidecarRef,
						},
					},
				},
				flux.MustParseResourceID("default:deployment/locked-service"): update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
					Error:  update.Locked,
				},
				flux.MustParseResourceID("default:deployment/test-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):  skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):       skippedNotInCluster,
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
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
				flux.MustParseResourceID(notInRepoService):                    skippedNotInRepo,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:   cluster,
				manifests: mockManifests,
				registry:  mockRegistry,
				repo:      checkout,
			}, tst.Spec, tst.Expected)
		})
	}
}

func Test_Force_lockedController(t *testing.T) {
	cluster := mockCluster(lockedSvc)
	success := update.ControllerResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{
			{
				Container: lockedContainer,
				Current:   oldLockedRef,
				Target:    newLockedRef,
			},
		},
	}
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		{
			Name: "force ignores service lock (--controller --update-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{lockedSvcSpec},
				ImageSpec:    update.ImageSpecFromRef(newLockedRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/locked-service"): success,
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
			},
		},
		{
			Name: "force does not ignore lock if updating all controllers (--all --update-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newLockedRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/locked-service"): skippedLocked,
				flux.MustParseResourceID("default:deployment/helloworld"):     skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/test-service"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):    skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):         skippedNotInCluster,
			},
		},
		{
			Name: "force ignores service lock (--controller --update-all-images)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{lockedSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/locked-service"): success,
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
			},
		},
		{
			Name: "force does not ignore lock if updating all controllers (--all --update-all-images)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/locked-service"): skippedLocked,
				flux.MustParseResourceID("default:deployment/helloworld"):     skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/test-service"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):    skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/semver"):         skippedNotInCluster,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:   cluster,
				manifests: mockManifests,
				registry:  mockRegistry,
				repo:      checkout,
			}, tst.Spec, tst.Expected)
		})
	}
}

func Test_Force_filteredContainer(t *testing.T) {
	cluster := mockCluster(semverSvc)
	successNew := update.ControllerResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{
			{
				Container: helloContainer,
				Current:   oldRef,
				Target:    newHwRef,
			},
		},
	}
	successSemver := update.ControllerResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{
			{
				Container: helloContainer,
				Current:   oldRef,
				Target:    semverHwRef,
			},
		},
	}
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseSpec
		Expected update.Result
	}{
		{
			Name: "force ignores container tag pattern (--controller --update-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{semverSvcSpec},
				ImageSpec:    update.ImageSpecFromRef(newHwRef), // does not match filter
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/semver"):         successNew,
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
			},
		},
		{
			Name: "force ignores container tag pattern (--all --update-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newHwRef), // does not match filter
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/semver"):         successNew,
				flux.MustParseResourceID("default:deployment/locked-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/helloworld"):     skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/test-service"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):    skippedNotInCluster,
			},
		},
		{
			Name: "force complies with semver when updating all images (--controller --update-all-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{semverSvcSpec},
				ImageSpec:    update.ImageSpecLatest, // will filter images by semver and pick newest version
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/semver"):         successSemver,
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
			},
		},
		{
			Name: "force complies with semver when updating all images (--all --update-all-image)",
			Spec: update.ReleaseSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []flux.ResourceID{},
				Force:        true,
			},
			Expected: update.Result{
				flux.MustParseResourceID("default:deployment/semver"):         successSemver,
				flux.MustParseResourceID("default:deployment/locked-service"): skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/helloworld"):     skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/test-service"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   skippedNotInCluster,
				flux.MustParseResourceID("default:deployment/list-deploy"):    skippedNotInCluster,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:   cluster,
				manifests: mockManifests,
				registry:  mockRegistry,
				repo:      checkout,
			}, tst.Spec, tst.Expected)
		})
	}
}

func Test_ImageStatus(t *testing.T) {
	cluster := mockCluster(hwSvc, lockedSvc, testSvc)
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
				flux.MustParseResourceID("default:deployment/helloworld"):     ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
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
				flux.MustParseResourceID("default:deployment/locked-service"): ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/test-service"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/multi-deploy"):   ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/list-deploy"):    ignoredNotIncluded,
				flux.MustParseResourceID("default:deployment/semver"):         ignoredNotIncluded,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			ctx := &ReleaseContext{
				cluster:   cluster,
				manifests: mockManifests,
				repo:      checkout,
				registry:  upToDateRegistry,
			}
			testRelease(t, ctx, tst.Spec, tst.Expected)
		})
	}
}

func Test_UpdateMultidoc(t *testing.T) {
	egID := flux.MustParseResourceID("default:deployment/multi-deploy")
	egSvc := cluster.Controller{
		ID: egID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  "hello",
					Image: oldRef,
				},
			},
		},
	}

	cluster := mockCluster(hwSvc, lockedSvc, egSvc) // no testsvc in cluster, but it _is_ in repo
	checkout, cleanup := setup(t)
	defer cleanup()
	ctx := &ReleaseContext{
		cluster:   cluster,
		manifests: mockManifests,
		repo:      checkout,
		registry:  mockRegistry,
	}
	spec := update.ReleaseSpec{
		ServiceSpecs: []update.ResourceSpec{"default:deployment/multi-deploy"},
		ImageSpec:    update.ImageSpecLatest,
		Kind:         update.ReleaseKindExecute,
	}
	results, err := Release(ctx, spec, log.NewNopLogger())
	if err != nil {
		t.Error(err)
	}
	controllerResult, ok := results[egID]
	if !ok {
		t.Fatal("controller not found after update")
	}
	if !reflect.DeepEqual(update.ControllerResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{{
			Container: "hello",
			Current:   oldRef,
			Target:    newHwRef,
		}},
	}, controllerResult) {
		t.Errorf("did not get expected controller result (see test code), got %#v", controllerResult)
	}
}

func Test_UpdateList(t *testing.T) {
	egID := flux.MustParseResourceID("default:deployment/list-deploy")
	egSvc := cluster.Controller{
		ID: egID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  "hello",
					Image: oldRef,
				},
			},
		},
	}

	cluster := mockCluster(hwSvc, lockedSvc, egSvc) // no testsvc in cluster, but it _is_ in repo
	checkout, cleanup := setup(t)
	defer cleanup()
	ctx := &ReleaseContext{
		cluster:   cluster,
		manifests: mockManifests,
		repo:      checkout,
		registry:  mockRegistry,
	}
	spec := update.ReleaseSpec{
		ServiceSpecs: []update.ResourceSpec{"default:deployment/list-deploy"},
		ImageSpec:    update.ImageSpecLatest,
		Kind:         update.ReleaseKindExecute,
	}
	results, err := Release(ctx, spec, log.NewNopLogger())
	if err != nil {
		t.Error(err)
	}
	controllerResult, ok := results[egID]
	if !ok {
		t.Fatal("controller not found after update")
	}
	if !reflect.DeepEqual(update.ControllerResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{{
			Container: "hello",
			Current:   oldRef,
			Target:    newHwRef,
		}},
	}, controllerResult) {
		t.Errorf("did not get expected controller result (see test code), got %#v", controllerResult)
	}
}

func Test_UpdateContainers(t *testing.T) {
	cluster := mockCluster(hwSvc, lockedSvc)
	checkout, cleanup := setup(t)
	defer cleanup()
	ctx := &ReleaseContext{
		cluster:   cluster,
		manifests: mockManifests,
		repo:      checkout,
		registry:  mockRegistry,
	}
	for _, tst := range []struct {
		Name string
		Spec []update.ContainerUpdate

		// true|false for `SkipMismatches`
		Commit   map[bool]string
		Expected map[bool]update.ControllerResult
		Err      map[bool]error
	}{
		{
			Name: "multiple containers",
			Spec: []update.ContainerUpdate{
				{
					Container: helloContainer,
					Current:   oldRef,
					Target:    newHwRef,
				},
				{
					Container: sidecarContainer,
					Current:   sidecarRef,
					Target:    newSidecarRef,
				},
			},
			Expected: map[bool]update.ControllerResult{
				true: {
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{{
						Container: helloContainer,
						Current:   oldRef,
						Target:    newHwRef,
					}, {
						Container: sidecarContainer,
						Current:   sidecarRef,
						Target:    newSidecarRef,
					}},
				},
				false: {
					Status: update.ReleaseStatusSuccess,
					PerContainer: []update.ContainerUpdate{{
						Container: helloContainer,
						Current:   oldRef,
						Target:    newHwRef,
					}, {
						Container: sidecarContainer,
						Current:   sidecarRef,
						Target:    newSidecarRef,
					}},
				},
			},
			Commit: map[bool]string{
				true:  "Release containers\n\ndefault:deployment/helloworld\n- quay.io/weaveworks/helloworld:master-a000002\n- weaveworks/sidecar:master-a000002\n",
				false: "Release containers\n\ndefault:deployment/helloworld\n- quay.io/weaveworks/helloworld:master-a000002\n- weaveworks/sidecar:master-a000002\n",
			},
		},
		{
			Name: "container tag mismatch",
			Spec: []update.ContainerUpdate{
				{
					Container: helloContainer,
					Current:   newHwRef, // mismatch
					Target:    oldRef,
				},
				{
					Container: sidecarContainer,
					Current:   sidecarRef,
					Target:    newSidecarRef,
				},
			},
			Expected: map[bool]update.ControllerResult{
				true: {
					Status: update.ReleaseStatusSuccess,
					Error:  fmt.Sprintf(update.ContainerTagMismatch, helloContainer),
					PerContainer: []update.ContainerUpdate{
						{
							Container: sidecarContainer,
							Current:   sidecarRef,
							Target:    newSidecarRef,
						},
					},
				},
				false: {}, // Errors.
			},
			Commit: map[bool]string{
				true: "Release containers\n\ndefault:deployment/helloworld\n- weaveworks/sidecar:master-a000002\n",
			},
		},
		{
			Name: "container not found",
			Spec: []update.ContainerUpdate{
				{
					Container: helloContainer,
					Current:   oldRef,
					Target:    newHwRef,
				},
				{
					Container: "foo", // not found
					Current:   oldRef,
					Target:    newHwRef,
				},
			},
			Expected: map[bool]update.ControllerResult{
				true:  {}, // Errors.
				false: {}, // Errors.
			},
		},
		{
			Name: "no changes",
			Spec: []update.ContainerUpdate{
				{
					Container: helloContainer,
					Current:   newHwRef, // mismatch
					Target:    newHwRef,
				},
			},
			Expected: map[bool]update.ControllerResult{
				true:  {},
				false: {},
			},
		},
	} {
		specs := update.ContainerSpecs{
			ContainerSpecs: map[flux.ResourceID][]update.ContainerUpdate{hwSvcID: tst.Spec},
			Kind:           update.ReleaseKindExecute,
		}

		for _, ignoreMismatches := range []bool{true, false} {
			name := tst.Name
			if ignoreMismatches {
				name += " (SkipMismatches)"
			}
			specs.SkipMismatches = ignoreMismatches
			results, err := Release(ctx, specs, log.NewNopLogger())
			if tst.Expected[ignoreMismatches].Status != "" {
				assert.Equal(t, tst.Expected[ignoreMismatches], results[hwSvcID], name)
				assert.Equal(t, tst.Commit[ignoreMismatches], specs.CommitMessage(results), name)
			} else {
				assert.Error(t, err, name)
			}
		}
	}
}

func testRelease(t *testing.T, ctx *ReleaseContext, spec update.ReleaseSpec, expected update.Result) {
	results, err := Release(ctx, spec, log.NewNopLogger())
	assert.NoError(t, err)
	assert.Equal(t, expected, results)
}

// --- test verification

// A Manifests implementation that does updates incorrectly, so they should fail verification.
type badManifests struct {
	kubernetes.Manifests
}

func (m *badManifests) UpdateImage(def []byte, resourceID flux.ResourceID, container string, newImageID image.Ref) ([]byte, error) {
	return def, nil
}

func Test_BadRelease(t *testing.T) {
	cluster := mockCluster(hwSvc)
	spec := update.ReleaseSpec{
		ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
		ImageSpec:    update.ImageSpecFromRef(newHwRef),
		Kind:         update.ReleaseKindExecute,
		Excludes:     []flux.ResourceID{},
	}
	checkout1, cleanup1 := setup(t)
	defer cleanup1()

	ctx := &ReleaseContext{
		cluster:   cluster,
		manifests: &kubernetes.Manifests{},
		repo:      checkout1,
		registry:  mockRegistry,
	}
	_, err := Release(ctx, spec, log.NewNopLogger())
	if err != nil {
		t.Fatal("release with 'good' Manifests should succeed, but errored:", err)
	}

	checkout2, cleanup2 := setup(t)
	defer cleanup2()

	ctx = &ReleaseContext{
		cluster:   cluster,
		manifests: &badManifests{Manifests: kubernetes.Manifests{}},
		repo:      checkout2,
		registry:  mockRegistry,
	}
	_, err = Release(ctx, spec, log.NewNopLogger())
	if err == nil {
		t.Fatal("did not return an error, but was expected to fail verification")
	}
}
