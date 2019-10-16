package v6

import (
	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/registry"
	"github.com/fluxcd/flux/pkg/update"
	"github.com/pkg/errors"
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

type imageSorter interface {
	// SortedImages returns the known images, sorted according to the
	// pattern given
	SortedImages(policy.Pattern) update.SortedImageInfos
	// Images returns the images in no defined order
	Images() []image.Info
}

// NewContainer creates a Container given a list of images and the current image
func NewContainer(name string, images imageSorter, currentImage image.Info, tagPattern policy.Pattern, fields []string) (Container, error) {
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

	// The following machinery attempts to minimise the number of
	// filters (`O(n)`) and sorts (`O(n log n)`), by memoising and
	// sharing intermediate results.

	var (
		sortedImages         update.SortedImageInfos
		filteredImages       []image.Info
		sortedFilteredImages update.SortedImageInfos
	)

	getFilteredImages := func() []image.Info {
		if filteredImages == nil {
			filteredImages = update.FilterImages(images.Images(), tagPattern)
		}
		return filteredImages
	}

	getSortedFilteredImages := func() update.SortedImageInfos {
		if sortedFilteredImages == nil {
			sortedFilteredImages = update.SortImages(getFilteredImages(), tagPattern)
		}
		return sortedFilteredImages
	}

	getSortedImages := func() update.SortedImageInfos {
		if sortedImages == nil {
			sortedImages = images.SortedImages(tagPattern)
			// now that we have the sorted images anyway, the fastest
			// way to get sorted, filtered images will be to filter
			// the already sorted images
			getSortedFilteredImages = func() update.SortedImageInfos {
				if sortedFilteredImages == nil {
					sortedFilteredImages = update.FilterImages(sortedImages, tagPattern)
				}
				return sortedFilteredImages
			}
			getFilteredImages = func() []image.Info {
				return []image.Info(getSortedFilteredImages())
			}
		}
		return sortedImages
	}

	// do these after we've gone through all the field names, since
	// they depend on what else is happening
	assignFields := []func(){}

	for _, field := range fields {
		switch field {
		// these first few rely only on the inputs
		case "Name":
			c.Name = name
		case "Current":
			c.Current = currentImage
		case "AvailableError":
			if images == nil {
				c.AvailableError = registry.ErrNoImageData.Error()
			}
		case "AvailableImagesCount":
			c.AvailableImagesCount = len(images.Images())

		// these required the sorted images, which we can get
		// straight away
		case "Available":
			c.Available = getSortedImages()
		case "NewAvailableImagesCount":
			newImagesCount := 0
			for _, img := range getSortedImages() {
				if !tagPattern.Newer(&img, &currentImage) {
					break
				}
				newImagesCount++
			}
			c.NewAvailableImagesCount = newImagesCount

		// these depend on what else gets calculated, so do them afterwards
		case "LatestFiltered": // needs sorted, filtered images
			assignFields = append(assignFields, func() {
				latest, _ := getSortedFilteredImages().Latest()
				c.LatestFiltered = latest
			})
		case "FilteredImagesCount": // needs filtered tags
			assignFields = append(assignFields, func() {
				c.FilteredImagesCount = len(getFilteredImages())
			})
		case "NewFilteredImagesCount": // needs filtered images
			assignFields = append(assignFields, func() {
				newFilteredImagesCount := 0
				for _, img := range getSortedFilteredImages() {
					if !tagPattern.Newer(&img, &currentImage) {
						break
					}
					newFilteredImagesCount++
				}
				c.NewFilteredImagesCount = newFilteredImagesCount
			})
		default:
			return c, errors.Errorf("%s is an invalid field", field)
		}
	}

	for _, fn := range assignFields {
		fn()
	}

	return c, nil
}
