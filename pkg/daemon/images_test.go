package daemon

import (
	"testing"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry"
	registryMock "github.com/fluxcd/flux/pkg/registry/mock"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

const (
	container1 = "container1"
	container2 = "container2"
	container3 = "container3"

	currentContainer1Image = "container1/application:current"
	newContainer1Image     = "container1/application:new"

	currentContainer2Image = "container2/application:current"
	newContainer2Image     = "container2/application:new"
	noTagContainer2Image   = "container2/application"

	currentContainer3Image = "container3/application:1.0.0"
	newContainer3Image     = "container3/application:1.1.0"
)

type candidate struct {
	resourceID resource.ID
	policies   policy.Set
}

func (c candidate) ResourceID() resource.ID {
	return c.resourceID
}

func (c candidate) Policies() policy.Set {
	return c.policies
}

func (candidate) Source() string {
	return ""
}

func (candidate) Bytes() []byte {
	return []byte{}
}

func TestCalculateChanges_Automated(t *testing.T) {
	logger := log.NewNopLogger()
	resourceID := resource.MakeID(ns, "deployment", "application")
	candidateWorkloads := resources{
		resourceID: candidate{
			resourceID: resourceID,
			policies: policy.Set{
				policy.Automated: "true",
			},
		},
	}
	workloads := []cluster.Workload{
		cluster.Workload{
			ID: resourceID,
			Containers: cluster.ContainersOrExcuse{
				Containers: []resource.Container{
					{
						Name:  container1,
						Image: mustParseImageRef(currentContainer1Image),
					},
				},
			},
		},
	}
	var imageRegistry registry.Registry
	{
		current := makeImageInfo(currentContainer1Image, time.Now())
		new := makeImageInfo(newContainer1Image, time.Now().Add(1*time.Second))
		imageRegistry = &registryMock.Registry{
			Images: []image.Info{
				current,
				new,
			},
		}
	}
	imageRepos, err := update.FetchImageRepos(imageRegistry, clusterContainers(workloads), logger)
	if err != nil {
		t.Fatal(err)
	}

	changes := calculateChanges(logger, candidateWorkloads, workloads, imageRepos)

	if len := len(changes.Changes); len != 1 {
		t.Errorf("Expected exactly 1 change, got %d changes", len)
	} else if newImage := changes.Changes[0].ImageID.String(); newImage != newContainer1Image {
		t.Errorf("Expected changed image to be %s, got %s", newContainer1Image, newImage)
	}
}
func TestCalculateChanges_UntaggedImage(t *testing.T) {
	logger := log.NewNopLogger()
	resourceID := resource.MakeID(ns, "deployment", "application")
	candidateWorkloads := resources{
		resourceID: candidate{
			resourceID: resourceID,
			policies: policy.Set{
				policy.Automated: "true",
			},
		},
	}
	workloads := []cluster.Workload{
		cluster.Workload{
			ID: resourceID,
			Containers: cluster.ContainersOrExcuse{
				Containers: []resource.Container{
					{
						Name:  container1,
						Image: mustParseImageRef(currentContainer1Image),
					},
					{
						Name:  container2,
						Image: mustParseImageRef(currentContainer2Image),
					},
				},
			},
		},
	}
	var imageRegistry registry.Registry
	{
		current1 := makeImageInfo(currentContainer1Image, time.Now())
		new1 := makeImageInfo(newContainer1Image, time.Now().Add(1*time.Second))
		current2 := makeImageInfo(currentContainer2Image, time.Now())
		noTag2 := makeImageInfo(noTagContainer2Image, time.Now().Add(1*time.Second))
		imageRegistry = &registryMock.Registry{
			Images: []image.Info{
				current1,
				new1,
				current2,
				noTag2,
			},
		}
	}
	imageRepos, err := update.FetchImageRepos(imageRegistry, clusterContainers(workloads), logger)
	if err != nil {
		t.Fatal(err)
	}

	changes := calculateChanges(logger, candidateWorkloads, workloads, imageRepos)

	if len := len(changes.Changes); len != 1 {
		t.Errorf("Expected exactly 1 change, got %d changes", len)
	} else if newImage := changes.Changes[0].ImageID.String(); newImage != newContainer1Image {
		t.Errorf("Expected changed image to be %s, got %s", newContainer1Image, newImage)
	}
}

func TestCalculateChanges_ZeroTimestamp(t *testing.T) {
	logger := log.NewNopLogger()
	resourceID := resource.MakeID(ns, "deployment", "application")
	candidateWorkloads := resources{
		resourceID: candidate{
			resourceID: resourceID,
			policies: policy.Set{
				policy.Automated:             "true",
				policy.TagPrefix(container3): "semver:^1.0",
			},
		},
	}
	workloads := []cluster.Workload{
		cluster.Workload{
			ID: resourceID,
			Containers: cluster.ContainersOrExcuse{
				Containers: []resource.Container{
					{
						Name:  container1,
						Image: mustParseImageRef(currentContainer1Image),
					},
					{
						Name:  container2,
						Image: mustParseImageRef(currentContainer2Image),
					},
					{
						Name:  container3,
						Image: mustParseImageRef(currentContainer3Image),
					},
				},
			},
		},
	}
	var imageRegistry registry.Registry
	{
		current1 := makeImageInfo(currentContainer1Image, time.Now())
		new1 := makeImageInfo(newContainer1Image, time.Now().Add(1*time.Second))

		zeroTimestampCurrent2 := image.Info{ID: mustParseImageRef(currentContainer2Image)}
		new2 := makeImageInfo(newContainer2Image, time.Now().Add(1*time.Second))

		current3 := makeImageInfo(currentContainer3Image, time.Now())
		zeroTimestampNew3 := image.Info{ID: mustParseImageRef(newContainer3Image)}

		imageRegistry = &registryMock.Registry{
			Images: []image.Info{
				current1,
				new1,
				zeroTimestampCurrent2,
				new2,
				current3,
				zeroTimestampNew3,
			},
		}
	}
	imageRepos, err := update.FetchImageRepos(imageRegistry, clusterContainers(workloads), logger)
	if err != nil {
		t.Fatal(err)
	}

	changes := calculateChanges(logger, candidateWorkloads, workloads, imageRepos)

	if len := len(changes.Changes); len != 2 {
		t.Fatalf("Expected exactly 2 changes, got %d changes: %v", len, changes.Changes)
	}
	if newImage := changes.Changes[0].ImageID.String(); newImage != newContainer1Image {
		t.Errorf("Expected changed image to be %s, got %s", newContainer1Image, newImage)
	}
	if newImage := changes.Changes[1].ImageID.String(); newImage != newContainer3Image {
		t.Errorf("Expected changed image to be %s, got %s", newContainer3Image, newImage)
	}
}
