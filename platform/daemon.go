package platform

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/registry"
)

// Combine these things to form Devasta^Wan implementation of
// Platform.
type Daemon struct {
	V        string
	Cluster  Cluster
	Registry registry.Registry
	Repo     git.Repo
}

// Invariant.
var _ Platform = &Daemon{}

func (d *Daemon) Version() (string, error) {
	return d.V, nil
}

func (d *Daemon) Ping() error {
	return d.Cluster.Ping()
}

func (d *Daemon) Export() ([]byte, error) {
	return d.Cluster.Export()
}

func (d *Daemon) ListServices(namespace string) ([]flux.ServiceStatus, error) {
	var res []flux.ServiceStatus
	services, err := d.Cluster.AllServices(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from cluster")
	}
	for _, service := range services {
		res = append(res, flux.ServiceStatus{
			ID:         service.ID,
			Containers: containers2containers(service.ContainersOrNil()),
			Status:     service.Status,
		})
	}
	return res, nil
}

// List the images available for set of services
func (d *Daemon) ListImages(spec flux.ServiceSpec) ([]flux.ImageStatus, error) {
	var services []Service
	var err error
	if spec == flux.ServiceSpecAll {
		services, err = d.Cluster.AllServices("")
	} else {
		id, err := spec.AsID()
		if err != nil {
			return nil, errors.Wrap(err, "treating service spec as ID")
		}
		services, err = d.Cluster.SomeServices([]flux.ServiceID{id})
	}

	images, err := CollectAvailableImages(d.Registry, services)
	if err != nil {
		return nil, errors.Wrap(err, "getting images for services")
	}

	var res []flux.ImageStatus
	for _, service := range services {
		containers := containersWithAvailable(service, images)
		res = append(res, flux.ImageStatus{
			ID:         service.ID,
			Containers: containers,
		})
	}

	return res, nil
}

// Apply the desired changes to the config files
func (d *Daemon) UpdateImages(flux.ReleaseSpec) (flux.ReleaseResult, error) {
	return nil, errors.New("FIXME")
}

// Tell the daemon to synchronise the cluster with the manifests in
// the git repo.
func (d *Daemon) SyncCluster() error {
	return errors.New("FIXME")
}

// Ask the daemon how far it's got applying things; in particular, is it
// past the supplied release? Return the list of commits between where
// we have applied and the ref given, inclusive. E.g., if you send HEAD,
// you'll get all the commits yet to be applied. If you send a hash
// and it's applied _past_ it, you'll get an empty list.
func (d *Daemon) SyncStatus(commitRef string) ([]string, error) {
	return nil, errors.New("FIXME")
}

// Non-platform.Platform methods

func (d *Daemon) LogEvent(ev flux.Event) error {
	// FIXME FIX FIXMEEEEEEE
	return nil
}

// vvv helpers vvv

func containers2containers(cs []Container) []flux.Container {
	res := make([]flux.Container, len(cs))
	for i, c := range cs {
		id, _ := flux.ParseImageID(c.Image)
		res[i] = flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
				ID: id,
			},
		}
	}
	return res
}

// For keeping track of which images are available
type ImageMap map[string][]flux.ImageDescription

// Create a map of images. It will check that each image exists.
func ExactImages(reg registry.Registry, images []flux.ImageID) (ImageMap, error) {
	m := ImageMap{}
	for _, id := range images {
		// We must check that the exact images requested actually exist. Otherwise we risk pushing invalid images to git.
		exist, err := imageExists(reg, id)
		if err != nil {
			return m, errors.Wrap(flux.ErrInvalidImageID, err.Error())
		}
		if !exist {
			return m, errors.Wrap(flux.ErrInvalidImageID, fmt.Sprintf("image %q does not exist", id))
		}
		m[id.Repository()] = []flux.ImageDescription{flux.ImageDescription{ID: id}}
	}
	return m, nil
}

// Checks whether the given image exists in the repository.
// Return true if exist, false otherwise
func imageExists(reg registry.Registry, imageID flux.ImageID) (bool, error) {
	// Use this method to parse the image, because it is safe. I.e. it will error and inform the user if it is malformed.
	img, err := flux.ParseImage(imageID.String(), nil)
	if err != nil {
		return false, err
	}
	// Get a specific image.
	_, err = reg.GetImage(registry.RepositoryFromImage(img), img.Tag)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Get the images available for the services given. An image may be
// mentioned more than once in the services, but will only be fetched
// once.
func CollectAvailableImages(reg registry.Registry, services []Service) (ImageMap, error) {
	images := ImageMap{}
	for _, service := range services {
		for _, container := range service.ContainersOrNil() {
			id, err := flux.ParseImageID(container.Image)
			if err != nil {
				// container is running an invalid image id? what?
				return nil, err
			}
			images[id.Repository()] = nil
		}
	}
	for repo := range images {
		r, err := registry.ParseRepository(repo)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing repository %s", repo)
		}
		imageRepo, err := reg.GetRepository(r)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching image metadata for %s", repo)
		}
		res := make([]flux.ImageDescription, len(imageRepo))
		for i, im := range imageRepo {
			id, err := flux.ParseImageID(im.String())
			if err != nil {
				// registry returned an invalid image id
				return nil, err
			}
			res[i] = flux.ImageDescription{
				ID:        id,
				CreatedAt: im.CreatedAt,
			}
		}
		images[repo] = res
	}
	return images, nil
}

// LatestImage returns the latest releasable image for a repository.
// A releasable image is one that is not tagged "latest". (Assumes the
// available images are in descending order of latestness.) If no such
// image exists, returns nil, and the caller can decide whether that's
// an error or not.
func (m ImageMap) LatestImage(repo string) *flux.ImageDescription {
	for _, image := range m[repo] {
		_, _, tag := image.ID.Components()
		if strings.EqualFold(tag, "latest") {
			continue
		}
		return &image
	}
	return nil
}

func containersWithAvailable(service Service, images ImageMap) (res []flux.Container) {
	for _, c := range service.ContainersOrNil() {
		id, _ := flux.ParseImageID(c.Image)
		repo := id.Repository()
		available := images[repo]
		res = append(res, flux.Container{
			Name: c.Name,
			Current: flux.ImageDescription{
				ID: id,
			},
			Available: available,
		})
	}
	return res
}
