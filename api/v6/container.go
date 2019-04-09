package v6

import (
	"github.com/pkg/errors"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/update"
)

// Container describes an individual container including current image info and
// available images.
type Container struct {
	Name           string     `json:",omitempty"`
	Current        image.Info `json:",omitempty"`
	LatestFiltered image.Info `json:",omitempty"`

	// All available images (ignoring tag filters)
	Available               update.SortedImageInfos `json:",omitempty"`
	AvailableError          string                  `json:",omitempty"`
	AvailableImagesCount    int                     `json:",omitempty"`
	NewAvailableImagesCount int                     `json:",omitempty"`

	// Filtered available images (matching tag filters)
	FilteredImagesCount    int `json:",omitempty"`
	NewFilteredImagesCount int `json:",omitempty"`
}

// NewContainer creates a Container given a list of images and the current image
func NewContainer(name string, images []image.Info, currentImage image.Info, tagPattern policy.Pattern, fields []string) (Container, error) {
	sorted := update.SortImages(images, tagPattern)

	// All images
	imagesCount := len(sorted)
	imagesErr := ""
	if sorted == nil {
		imagesErr = registry.ErrNoImageData.Error()
	}
	var newImages update.SortedImageInfos
	for _, img := range sorted {
		if tagPattern.Newer(&img, &currentImage) {
			newImages = append(newImages, img)
		}
	}
	newImagesCount := len(newImages)

	// Filtered images (which respects sorting)
	filteredImages := update.SortedImageInfos(update.FilterImages(sorted, tagPattern))
	filteredImagesCount := len(filteredImages)
	var newFilteredImages update.SortedImageInfos
	for _, img := range filteredImages {
		if tagPattern.Newer(&img, &currentImage) {
			newFilteredImages = append(newFilteredImages, img)
		}
	}
	newFilteredImagesCount := len(newFilteredImages)
	latestFiltered, _ := filteredImages.Latest()

	container := Container{
		Name:           name,
		Current:        currentImage,
		LatestFiltered: latestFiltered,

		Available:               sorted,
		AvailableError:          imagesErr,
		AvailableImagesCount:    imagesCount,
		NewAvailableImagesCount: newImagesCount,
		FilteredImagesCount:     filteredImagesCount,
		NewFilteredImagesCount:  newFilteredImagesCount,
	}
	return filterContainerFields(container, fields)
}

// filterContainerFields returns a new container with only the fields specified. If not fields are specified,
// a list of default fields is used.
func filterContainerFields(container Container, fields []string) (Container, error) {
	// Default fields
	if len(fields) == 0 {
		fields = []string{
			"Name",
			"Current",
			"LatestFiltered",
			"Available",
			"AvailableError",
			"AvailableImagesCount",
			"NewAvailableImagesCount",
			"FilteredImagesCount",
			"NewFilteredImagesCount",
		}
	}

	var c Container
	for _, field := range fields {
		switch field {
		case "Name":
			c.Name = container.Name
		case "Current":
			c.Current = container.Current
		case "LatestFiltered":
			c.LatestFiltered = container.LatestFiltered
		case "Available":
			c.Available = container.Available
		case "AvailableError":
			c.AvailableError = container.AvailableError
		case "AvailableImagesCount":
			c.AvailableImagesCount = container.AvailableImagesCount
		case "NewAvailableImagesCount":
			c.NewAvailableImagesCount = container.NewAvailableImagesCount
		case "FilteredImagesCount":
			c.FilteredImagesCount = container.FilteredImagesCount
		case "NewFilteredImagesCount":
			c.NewFilteredImagesCount = container.NewFilteredImagesCount
		default:
			return c, errors.Errorf("%s is an invalid field", field)
		}
	}
	return c, nil
}
