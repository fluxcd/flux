package instance

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"
)

type Instancer interface {
	Get(inst flux.InstanceID) (*Instance, error)
}

type Instance struct {
	platform platform.Platform
	registry registry.Registry
	config   Configurer
	duration metrics.Histogram
	gitrepo  git.Repo

	log.Logger
	history.EventReader
	history.EventWriter
}

func New(
	platform platform.Platform,
	registry registry.Registry,
	config Configurer,
	gitrepo git.Repo,
	logger log.Logger,
	duration metrics.Histogram,
	events history.EventReader,
	eventlog history.EventWriter,
) *Instance {
	return &Instance{
		platform:    platform,
		registry:    registry,
		config:      config,
		gitrepo:     gitrepo,
		duration:    duration,
		Logger:      logger,
		EventReader: events,
		EventWriter: eventlog,
	}
}

func (h *Instance) ConfigRepo() git.Repo {
	return h.gitrepo
}

type ImageMap map[string][]flux.ImageDescription

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

// Get the services in `namespace` along with their containers (if
// there are any) from the platform; if namespace is blank, just get
// all the services, in any namespace.
func (h *Instance) GetAllServices(maybeNamespace string) ([]platform.Service, error) {
	return h.GetAllServicesExcept(maybeNamespace, flux.ServiceIDSet{})
}

// Get all services except those with an ID in the set given
func (h *Instance) GetAllServicesExcept(maybeNamespace string, ignored flux.ServiceIDSet) (res []platform.Service, err error) {
	return h.platform.AllServices(maybeNamespace, ignored)
}

// Get the services mentioned, along with their containers.
func (h *Instance) GetServices(ids []flux.ServiceID) ([]platform.Service, error) {
	return h.platform.SomeServices(ids)
}

// Get the images available for the services given. An image may be
// mentioned more than once in the services, but will only be fetched
// once.
func (h *Instance) CollectAvailableImages(services []platform.Service) (ImageMap, error) {
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
		imageRepo, err := h.registry.GetRepository(r)
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

// GetRepository exposes this instance's registry's GetRepository method directly.
func (h *Instance) GetRepository(repo string) (res []flux.ImageDescription, err error) {
	r, err := registry.ParseRepository(repo)
	if err != nil {
		return
	}
	images, err := h.registry.GetRepository(r)
	if err != nil {
		return
	}
	res = make([]flux.ImageDescription, len(images))
	for i, im := range images {
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
	return
}

// Create a map of images. It will check that each image exists.
func (h *Instance) ExactImages(images []flux.ImageID) (ImageMap, error) {
	m := ImageMap{}
	for _, id := range images {
		// We must check that the exact images requested actually exist. Otherwise we risk pushing invalid images to git.
		exist, err := h.imageExists(id)
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
func (h *Instance) imageExists(imageID flux.ImageID) (bool, error) {
	// Use this method to parse the image, because it is safe. I.e. it will error and inform the user if it is malformed.
	img, err := flux.ParseImage(imageID.String(), nil)
	if err != nil {
		return false, err
	}
	// Get a specific image.
	_, err = h.registry.GetImage(registry.RepositoryFromImage(img), img.Tag)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (h *Instance) PlatformApply(defs []platform.ServiceDefinition) (err error) {
	defer func(begin time.Time) {
		h.duration.With(
			fluxmetrics.LabelMethod, "PlatformApply",
			fluxmetrics.LabelSuccess, fmt.Sprint(err == nil),
		).Observe(time.Since(begin).Seconds())
	}(time.Now())

	return h.platform.Apply(defs)
}

func (h *Instance) Ping() error {
	return h.platform.Ping()
}

func (h *Instance) Version() (string, error) {
	return h.platform.Version()
}

func (h *Instance) GetConfig() (Config, error) {
	return h.config.Get()
}

func (h *Instance) UpdateConfig(update UpdateFunc) error {
	return h.config.Update(update)
}
