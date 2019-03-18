package daemon

import (
	"github.com/weaveworks/flux/policy"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/registry"
	registryMock "github.com/weaveworks/flux/registry/mock"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

const (
	container1 = "container1"
	container2 = "container2"

	currentContainer1Image = "container1/application:current"
	newContainer1Image     = "container1/application:new"

	currentContainer2Image = "container2/application:current"
	newContainer2Image     = "container2/application:new"
	noTagContainer2Image   = "container2/application"
)

type candidate struct {
	resourceID flux.ResourceID
	policies   policy.Set
}

func (c candidate) ResourceID() flux.ResourceID {
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
	resourceID := flux.MakeResourceID(ns, "deployment", "application")
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
	resourceID := flux.MakeResourceID(ns, "deployment", "application")
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
	resourceID := flux.MakeResourceID(ns, "deployment", "application")
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
		zeroTimestampCurrent2 := image.Info{ID: mustParseImageRef(currentContainer2Image)}
		new2 := makeImageInfo(newContainer2Image, time.Now().Add(1*time.Second))
		imageRegistry = &registryMock.Registry{
			Images: []image.Info{
				current1,
				new1,
				zeroTimestampCurrent2,
				new2,
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
