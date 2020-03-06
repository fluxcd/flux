package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/update"

	v6 "github.com/fluxcd/flux/pkg/api/v6"
)

func Test_outputImagesJson(t *testing.T) {
	opts := &imageListOpts{limit: 10} // 10 is the default from the flag

	t.Run("sends JSON to the io.Writer", func(t *testing.T) {
		buf := &bytes.Buffer{}
		images := testImages(opts.limit)
		err := outputImagesJson(images, buf, opts)
		require.NoError(t, err)
		unmarshallTarget := &[]v6.ImageStatus{}
		err = json.Unmarshal(buf.Bytes(), unmarshallTarget)
		require.NoError(t, err)
	})

	t.Run("respects provided limit on Available container images", func(t *testing.T) {
		buf := &bytes.Buffer{}
		images := testImages(100)
		_ = outputImagesJson(images, buf, opts)

		unmarshallTarget := &[]v6.ImageStatus{}
		_ = json.Unmarshal(buf.Bytes(), unmarshallTarget)

		imageSlice := *unmarshallTarget
		availableListSize := len(imageSlice[0].Containers[0].Available)

		assert.Equal(t, opts.limit, availableListSize)
	})

	t.Run("provides all when limit is 0", func(t *testing.T) {
		buf := &bytes.Buffer{}
		opts := &imageListOpts{limit: 0} // 0 means all
		count := 100
		images := testImages(count)
		_ = outputImagesJson(images, buf, opts)

		unmarshallTarget := &[]v6.ImageStatus{}
		_ = json.Unmarshal(buf.Bytes(), unmarshallTarget)

		imageSlice := *unmarshallTarget
		availableListSize := len(imageSlice[0].Containers[0].Available)

		assert.Equal(t, count, availableListSize)
	})

	t.Run("returns an error on a bad limit", func(t *testing.T) {
		badLimitOpts := &imageListOpts{limit: -1}
		buf := &bytes.Buffer{}
		images := testImages(10)
		err := outputImagesJson(images, buf, badLimitOpts)
		assert.Error(t, err)
	})
}

// testImages returns a single-member collection of ImageStatus objects with
// an optional number of Available images on the only Container
func testImages(availableCount int) []v6.ImageStatus {
	containerWithAvailable := []v6.Container{{Name: "TestContainer"}}

	images := []v6.ImageStatus{{Containers: containerWithAvailable}}
	available := update.SortedImageInfos{}

	for i := 0; i < availableCount; i++ {
		digest := fmt.Sprintf("abc123%d", i)
		imageID := fmt.Sprintf("deadbeef%d", i)
		testImage := image.Info{
			ID:          image.Ref{},
			Digest:      digest,
			ImageID:     imageID,
			Labels:      image.Labels{},
			CreatedAt:   time.Time{},
			LastFetched: time.Time{},
		}

		available = append(available, testImage)
	}

	images[0].Containers[0].Available = available

	return images
}
