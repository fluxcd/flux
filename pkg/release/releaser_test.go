package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/cluster/kubernetes"
	"github.com/fluxcd/flux/pkg/cluster/mock"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/git/gittest"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/manifests"
	registryMock "github.com/fluxcd/flux/pkg/registry/mock"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
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
	hwSvcID, _    = resource.ParseID("default:deployment/helloworld")
	hwSvcSpec, _  = update.ParseResourceSpec(hwSvcID.String())
	hwSvc         = cluster.Workload{
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
	lockedSvcID, _   = resource.ParseID("default:deployment/locked-service")
	lockedSvcSpec, _ = update.ParseResourceSpec(lockedSvcID.String())
	lockedSvc        = cluster.Workload{
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
	semverSvcID    = resource.MustParseID("default:deployment/semver")
	semverSvc      = cluster.Workload{
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

	testSvcID = resource.MustParseID("default:deployment/test-service")
	testSvc   = cluster.Workload{
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

	allSvcs = []cluster.Workload{
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
	mockManifests = kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
)

func mockCluster(running ...cluster.Workload) *mock.Mock {
	return &mock.Mock{
		AllWorkloadsFunc: func(ctx context.Context, maybeNamespace string) ([]cluster.Workload, error) {
			return running, nil
		},
		SomeWorkloadsFunc: func(ctx context.Context, ids []resource.ID) ([]cluster.Workload, error) {
			var res []cluster.Workload
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

func NewManifestStoreOrFail(t *testing.T, parser manifests.Manifests, checkout *git.Checkout) manifests.Store {
	cm := manifests.NewRawFiles(checkout.Dir(), checkout.AbsolutePaths(), parser)
	return cm
}

func setup(t *testing.T) (*git.Checkout, func()) {
	return gittest.Checkout(t)
}

var ignoredNotIncluded = update.WorkloadResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotIncluded,
}

var ignoredNotInRepo = update.WorkloadResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotInRepo,
}

var ignoredNotInCluster = update.WorkloadResult{
	Status: update.ReleaseStatusIgnored,
	Error:  update.NotAccessibleInCluster,
}

var skippedLocked = update.WorkloadResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.Locked,
}

var skippedNotInCluster = update.WorkloadResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.NotAccessibleInCluster,
}

var skippedNotInRepo = update.WorkloadResult{
	Status: update.ReleaseStatusSkipped,
	Error:  update.NotInRepo,
}

type expected struct {
	// The result we care about
	Specific update.Result
	// What everything not mentioned gets
	Else update.WorkloadResult
}

// Result returns the expected result taking into account what is
// specified with Specific and what is elided with Else
func (x expected) Result() update.Result {
	result := x.Specific
	for _, id := range gittest.Workloads() {
		if _, ok := result[id]; !ok {
			result[id] = x.Else
		}
	}
	return result
}

func Test_InitContainer(t *testing.T) {
	initWorkloadID := resource.MustParseID("default:daemonset/init")
	initSvc := cluster.Workload{
		ID: initWorkloadID,
		Containers: cluster.ContainersOrExcuse{
			Containers: []resource.Container{
				{
					Name:  helloContainer,
					Image: oldRef,
				},
			},
		},
	}

	mCluster := mockCluster(hwSvc, lockedSvc, initSvc)

	expect := expected{
		Specific: update.Result{
			initWorkloadID: update.WorkloadResult{
				Status: update.ReleaseStatusSuccess,
				PerContainer: []update.ContainerUpdate{
					{
						Container: helloContainer,
						Current:   oldRef,
						Target:    newHwRef,
					},
				},
			},
		},
		Else: ignoredNotIncluded,
	}

	initSpec, _ := update.ParseResourceSpec(initWorkloadID.String())
	spec := update.ReleaseImageSpec{
		ServiceSpecs: []update.ResourceSpec{initSpec},
		ImageSpec:    update.ImageSpecLatest,
		Kind:         update.ReleaseKindExecute,
	}

	checkout, clean := setup(t)
	defer clean()

	testRelease(t, &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
		registry:      mockRegistry,
	}, spec, expect.Result())

}

func Test_FilterLogic(t *testing.T) {
	mCluster := mockCluster(hwSvc, lockedSvc) // no testsvc in cluster, but it _is_ in repo

	notInRepoService := "default:deployment/notInRepo"
	notInRepoSpec, _ := update.ParseResourceSpec(notInRepoService)
	for _, tst := range []struct {
		Name     string
		Spec     update.ReleaseImageSpec
		Expected expected
	}{
		// ignored if: excluded OR not included OR not correct image.
		{
			Name: "include specific service",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
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
					},
				},
				Else: ignoredNotIncluded,
			},
		}, {
			Name: "exclude specific service",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{lockedSvcID},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
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
					},
					resource.MustParseID("default:deployment/locked-service"): update.WorkloadResult{
						Status: update.ReleaseStatusIgnored,
						Error:  update.Excluded,
					},
				},
				Else: skippedNotInCluster,
			},
		}, {
			Name: "update specific image",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newHwRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
							{
								Container: helloContainer,
								Current:   oldRef,
								Target:    newHwRef,
							},
						},
					},
					resource.MustParseID("default:deployment/locked-service"): update.WorkloadResult{
						Status: update.ReleaseStatusIgnored,
						Error:  update.DifferentImage,
					},
				},
				Else: skippedNotInCluster,
			},
		}, {
			// skipped if: not ignored AND (locked or not found in cluster)
			// else: service is pending.
			Name: "skipped & service is pending",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
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
					},
					resource.MustParseID("default:deployment/locked-service"): update.WorkloadResult{
						Status: update.ReleaseStatusSkipped,
						Error:  update.Locked,
					},
				},
				Else: skippedNotInCluster,
			},
		}, {
			Name: "all overrides spec",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec, update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
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
					},
					resource.MustParseID("default:deployment/locked-service"): update.WorkloadResult{
						Status: update.ReleaseStatusSkipped,
						Error:  update.Locked,
					},
				},
				Else: skippedNotInCluster,
			},
		}, {
			Name: "service not in repo",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{notInRepoSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID(notInRepoService): skippedNotInRepo,
				},
				Else: ignoredNotIncluded,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:       mCluster,
				resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
				registry:      mockRegistry,
			}, tst.Spec, tst.Expected.Result())
		})
	}
}

func Test_Force_lockedWorkload(t *testing.T) {
	mCluster := mockCluster(lockedSvc)
	success := update.WorkloadResult{
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
		Spec     update.ReleaseImageSpec
		Expected expected
	}{
		{
			Name: "force ignores service lock (--workload --update-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{lockedSvcSpec},
				ImageSpec:    update.ImageSpecFromRef(newLockedRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/locked-service"): success,
				},
				Else: ignoredNotIncluded,
			},
		}, {
			Name: "force does not ignore lock if updating all workloads (--all --update-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newLockedRef),
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/locked-service"): skippedLocked,
				},
				Else: skippedNotInCluster,
			},
		},
		{
			Name: "force ignores service lock (--workload --update-all-images)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{lockedSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/locked-service"): success,
				},
				Else: ignoredNotIncluded,
			},
		}, {
			Name: "force does not ignore lock if updating all workloads (--all --update-all-images)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/locked-service"): skippedLocked,
				},
				Else: skippedNotInCluster,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:       mCluster,
				resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
				registry:      mockRegistry,
			}, tst.Spec, tst.Expected.Result())
		})
	}
}

func Test_Force_filteredContainer(t *testing.T) {
	mCluster := mockCluster(semverSvc)
	successNew := update.WorkloadResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{
			{
				Container: helloContainer,
				Current:   oldRef,
				Target:    newHwRef,
			},
		},
	}
	successSemver := update.WorkloadResult{
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
		Spec     update.ReleaseImageSpec
		Expected expected
	}{
		{
			Name: "force ignores container tag pattern (--workload --update-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{semverSvcSpec},
				ImageSpec:    update.ImageSpecFromRef(newHwRef), // does not match filter
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/semver"): successNew,
				},
				Else: ignoredNotIncluded,
			},
		},
		{
			Name: "force ignores container tag pattern (--all --update-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecFromRef(newHwRef), // does not match filter
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/semver"): successNew,
				},
				Else: skippedNotInCluster,
			},
		},
		{
			Name: "force complies with semver when updating all images (--workload --update-all-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{semverSvcSpec},
				ImageSpec:    update.ImageSpecLatest, // will filter images by semver and pick newest version
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/semver"): successSemver,
				},
				Else: ignoredNotIncluded,
			},
		},
		{
			Name: "force complies with semver when updating all images (--all --update-all-image)",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
				Force:        true,
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/semver"): successSemver,
				},
				Else: skippedNotInCluster,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			testRelease(t, &ReleaseContext{
				cluster:       mCluster,
				resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
				registry:      mockRegistry,
			}, tst.Spec, tst.Expected.Result())
		})
	}
}

func Test_ImageStatus(t *testing.T) {
	mCluster := mockCluster(hwSvc, lockedSvc, testSvc)
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
		Spec     update.ReleaseImageSpec
		Expected expected
	}{
		{
			Name: "image not found",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{testSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/test-service"): update.WorkloadResult{
						Status: update.ReleaseStatusIgnored,
						Error:  update.DoesNotUseImage,
					},
				},
				Else: ignoredNotIncluded,
			},
		}, {
			Name: "image up to date",
			Spec: update.ReleaseImageSpec{
				ServiceSpecs: []update.ResourceSpec{hwSvcSpec},
				ImageSpec:    update.ImageSpecLatest,
				Kind:         update.ReleaseKindExecute,
				Excludes:     []resource.ID{},
			},
			Expected: expected{
				Specific: update.Result{
					resource.MustParseID("default:deployment/helloworld"): update.WorkloadResult{
						Status: update.ReleaseStatusSkipped,
						Error:  update.ImageUpToDate,
					},
				},
				Else: ignoredNotIncluded,
			},
		},
	} {
		t.Run(tst.Name, func(t *testing.T) {
			checkout, cleanup := setup(t)
			defer cleanup()
			rc := &ReleaseContext{
				cluster:       mCluster,
				resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
				registry:      upToDateRegistry,
			}
			testRelease(t, rc, tst.Spec, tst.Expected.Result())
		})
	}
}

func Test_UpdateMultidoc(t *testing.T) {
	egID := resource.MustParseID("default:deployment/multi-deploy")
	egSvc := cluster.Workload{
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

	mCluster := mockCluster(hwSvc, lockedSvc, egSvc) // no testsvc in cluster, but it _is_ in repo
	checkout, cleanup := setup(t)
	defer cleanup()
	rc := &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
		registry:      mockRegistry,
	}
	spec := update.ReleaseImageSpec{
		ServiceSpecs: []update.ResourceSpec{"default:deployment/multi-deploy"},
		ImageSpec:    update.ImageSpecLatest,
		Kind:         update.ReleaseKindExecute,
	}
	results, err := Release(context.Background(), rc, spec, log.NewNopLogger())
	if err != nil {
		t.Error(err)
	}
	workloadResult, ok := results[egID]
	if !ok {
		t.Fatal("workload not found after update")
	}
	if !reflect.DeepEqual(update.WorkloadResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{{
			Container: "hello",
			Current:   oldRef,
			Target:    newHwRef,
		}},
	}, workloadResult) {
		t.Errorf("did not get expected workload result (see test code), got %#v", workloadResult)
	}
}

func Test_UpdateList(t *testing.T) {
	egID := resource.MustParseID("default:deployment/list-deploy")
	egSvc := cluster.Workload{
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

	mCluster := mockCluster(hwSvc, lockedSvc, egSvc) // no testsvc in cluster, but it _is_ in repo
	checkout, cleanup := setup(t)
	defer cleanup()
	rc := &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
		registry:      mockRegistry,
	}
	spec := update.ReleaseImageSpec{
		ServiceSpecs: []update.ResourceSpec{"default:deployment/list-deploy"},
		ImageSpec:    update.ImageSpecLatest,
		Kind:         update.ReleaseKindExecute,
	}
	results, err := Release(context.Background(), rc, spec, log.NewNopLogger())
	if err != nil {
		t.Error(err)
	}
	workloadResult, ok := results[egID]
	if !ok {
		t.Fatal("workload not found after update")
	}
	if !reflect.DeepEqual(update.WorkloadResult{
		Status: update.ReleaseStatusSuccess,
		PerContainer: []update.ContainerUpdate{{
			Container: "hello",
			Current:   oldRef,
			Target:    newHwRef,
		}},
	}, workloadResult) {
		t.Errorf("did not get expected workload result (see test code), got %#v", workloadResult)
	}
}

func Test_UpdateContainers(t *testing.T) {
	mCluster := mockCluster(hwSvc, lockedSvc)
	checkout, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()
	rc := &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, mockManifests, checkout),
		registry:      mockRegistry,
	}
	type expected struct {
		Err    error
		Result update.WorkloadResult
		Commit string
	}
	for _, tst := range []struct {
		Name       string
		WorkloadID resource.ID
		Spec       []update.ContainerUpdate
		Force      bool

		SkipMismatches map[bool]expected
	}{
		{
			Name:       "multiple containers",
			WorkloadID: hwSvcID,
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
			SkipMismatches: map[bool]expected{
				true: {
					Result: update.WorkloadResult{
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
					Commit: "Update image refs in default:deployment/helloworld\n\ndefault:deployment/helloworld\n- quay.io/weaveworks/helloworld:master-a000002\n- weaveworks/sidecar:master-a000002\n",
				},
				false: {
					Result: update.WorkloadResult{
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
					Commit: "Update image refs in default:deployment/helloworld\n\ndefault:deployment/helloworld\n- quay.io/weaveworks/helloworld:master-a000002\n- weaveworks/sidecar:master-a000002\n",
				},
			},
		},
		{
			Name:       "container tag mismatch",
			WorkloadID: hwSvcID,
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
			SkipMismatches: map[bool]expected{
				true: {
					Result: update.WorkloadResult{
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
					Commit: "Update image refs in default:deployment/helloworld\n\ndefault:deployment/helloworld\n- weaveworks/sidecar:master-a000002\n",
				},
				false: {Err: errors.New("cannot satisfy specs")},
			},
		},
		{
			Name:       "container not found",
			WorkloadID: hwSvcID,
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
			SkipMismatches: map[bool]expected{
				true:  {Err: errors.New("cannot satisfy specs")},
				false: {Err: errors.New("cannot satisfy specs")},
			},
		},
		{
			Name:       "no changes",
			WorkloadID: hwSvcID,
			Spec: []update.ContainerUpdate{
				{
					Container: helloContainer,
					Current:   oldRef,
					Target:    oldRef,
				},
			},
			SkipMismatches: map[bool]expected{
				true:  {Err: errors.New("no changes found")},
				false: {Err: errors.New("no changes found")},
			},
		},
		{
			Name:       "locked workload",
			WorkloadID: lockedSvcID,
			Spec: []update.ContainerUpdate{
				{ // This is valid but as the workload is locked, there won't be any changes.
					Container: lockedContainer,
					Current:   oldLockedRef,
					Target:    newLockedRef,
				},
			},
			SkipMismatches: map[bool]expected{
				true:  {Err: errors.New("no changes found")},
				false: {Err: errors.New("no changes found")},
			},
		},
		{
			Name:       "locked workload with --force",
			WorkloadID: lockedSvcID,
			Force:      true,
			Spec: []update.ContainerUpdate{
				{ // The workload is locked but lock flag is ignored.
					Container: lockedContainer,
					Current:   oldLockedRef,
					Target:    newLockedRef,
				},
			},
			SkipMismatches: map[bool]expected{
				true: {
					Result: update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
							{
								Container: lockedContainer,
								Current:   oldLockedRef,
								Target:    newLockedRef,
							},
						},
					},
					Commit: "Update image refs in default:deployment/locked-service\n\ndefault:deployment/locked-service\n- quay.io/weaveworks/locked-service:2\n",
				},
				false: {
					Result: update.WorkloadResult{
						Status: update.ReleaseStatusSuccess,
						PerContainer: []update.ContainerUpdate{
							{
								Container: lockedContainer,
								Current:   oldLockedRef,
								Target:    newLockedRef,
							},
						},
					},
					Commit: "Update image refs in default:deployment/locked-service\n\ndefault:deployment/locked-service\n- quay.io/weaveworks/locked-service:2\n",
				},
			},
		},
	} {
		specs := update.ReleaseContainersSpec{
			ContainerSpecs: map[resource.ID][]update.ContainerUpdate{tst.WorkloadID: tst.Spec},
			Kind:           update.ReleaseKindExecute,
		}

		for ignoreMismatches, expected := range tst.SkipMismatches {
			name := tst.Name
			if ignoreMismatches {
				name += " (SkipMismatches)"
			}
			t.Run(name, func(t *testing.T) {
				specs.SkipMismatches = ignoreMismatches
				specs.Force = tst.Force

				results, err := Release(ctx, rc, specs, log.NewNopLogger())

				assert.Equal(t, expected.Err, err)
				if expected.Err == nil {
					assert.Equal(t, expected.Result, results[tst.WorkloadID])
					assert.Equal(t, expected.Commit, specs.CommitMessage(results))
				}
			})
		}
	}
}

func testRelease(t *testing.T, rc *ReleaseContext, spec update.ReleaseImageSpec, expected update.Result) {
	results, err := Release(context.Background(), rc, spec, log.NewNopLogger())
	assert.NoError(t, err)
	assert.Equal(t, expected, results)
}

// --- test verification

// A manifests implementation that does updates incorrectly, so they should fail verification.
type badManifests struct {
	manifests.Manifests
}

func (m *badManifests) SetWorkloadContainerImage(def []byte, id resource.ID, container string, image image.Ref) ([]byte, error) {
	return def, nil
}

func Test_BadRelease(t *testing.T) {
	mCluster := mockCluster(hwSvc)
	spec := update.ReleaseImageSpec{
		ServiceSpecs: []update.ResourceSpec{update.ResourceSpecAll},
		ImageSpec:    update.ImageSpecFromRef(newHwRef),
		Kind:         update.ReleaseKindExecute,
		Excludes:     []resource.ID{},
	}
	checkout1, cleanup1 := setup(t)
	defer cleanup1()

	manifests := kubernetes.NewManifests(kubernetes.ConstNamespacer("default"), log.NewLogfmtLogger(os.Stdout))
	ctx := context.Background()
	rc := &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, manifests, checkout1),
		registry:      mockRegistry,
	}
	_, err := Release(ctx, rc, spec, log.NewNopLogger())
	if err != nil {
		t.Fatal("release with 'good' manifests should succeed, but errored:", err)
	}

	checkout2, cleanup2 := setup(t)
	defer cleanup2()

	rc = &ReleaseContext{
		cluster:       mCluster,
		resourceStore: NewManifestStoreOrFail(t, &badManifests{manifests}, checkout2),
		registry:      mockRegistry,
	}
	_, err = Release(ctx, rc, spec, log.NewNopLogger())
	if err == nil {
		t.Fatal("did not return an error, but was expected to fail verification")
	}
}
