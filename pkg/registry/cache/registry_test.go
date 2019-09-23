package cache

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/stretchr/testify/assert"
)

// mockStorage holds a fixed ImageRepository item.
type mockStorage struct {
	Item ImageRepository
}

// GetKey will always return the same item from the storage,
// and does not care about the key it receives.
func (m *mockStorage) GetKey(k Keyer) ([]byte, time.Time, error) {
	b, err := json.Marshal(m.Item)
	if err != nil {
		return []byte{}, time.Time{}, err
	}
	return b, time.Time{}, nil
}

// appendImage adds an image to the mocked storage item.
func (m *mockStorage) appendImage(i image.Info) {
	tag := i.ID.Tag

	m.Item.Images[tag] = i
	m.Item.Tags = append(m.Item.Tags, tag)
}

func mockReader() *mockStorage {
	return &mockStorage{
		Item: ImageRepository{
			RepositoryMetadata: image.RepositoryMetadata{
				Tags:   []string{},
				Images: map[string]image.Info{},
			},
			LastUpdate: time.Now(),
		},
	}
}

func Test_WhitelabelDecorator(t *testing.T) {
	r := mockReader()

	// Image with no timestamp label
	r.appendImage(mustMakeInfo("docker.io/fluxcd/flux:equal", time.Time{}, time.Now().UTC()))
	// Image with a timestamp label
	r.appendImage(mustMakeInfo("docker.io/fluxcd/flux:label", time.Now().Add(-10*time.Second).UTC(), time.Now().UTC()))

	c := Cache{r, []Decorator{TimestampLabelWhitelist{"index.docker.io/fluxcd/*"}}}

	rm, err := c.GetImageRepositoryMetadata(image.Name{})
	assert.NoError(t, err)

	assert.Equal(t, r.Item.Images["equal"].CreatedAt, rm.Images["equal"].CreatedAt)
	assert.Equal(t, r.Item.Images["label"].Labels.Created, rm.Images["label"].CreatedAt)
}

func mustMakeInfo(ref string, label time.Time, created time.Time) image.Info {
	r, err := image.ParseRef(ref)
	if err != nil {
		panic(err)
	}
	var labels image.Labels
	if !label.IsZero() {
		labels.Created = label
	}
	return image.Info{ID: r, Labels: labels, CreatedAt: created}
}
