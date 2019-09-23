package registry

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
)

// Check that the ImageEntry type can be round-tripped via JSON.
func TestImageEntryRoundtrip(t *testing.T) {

	test := func(t *testing.T, entry ImageEntry) {
		bytes, err := json.Marshal(entry)
		assert.NoError(t, err)

		var entry2 ImageEntry
		assert.NoError(t, json.Unmarshal(bytes, &entry2))
		assert.Equal(t, entry, entry2)
	}

	ref, err := image.ParseRef("docker.io/fluxcd/flux:1.0.0")
	assert.NoError(t, err)

	info := image.Info{
		ID:        ref,
		CreatedAt: time.Now().UTC(), // to UTC since we unmarshal times in UTC
	}

	entry := ImageEntry{
		Info: info,
	}
	t.Run("With an info", func(t *testing.T) { test(t, entry) })
	t.Run("With an excluded reason", func(t *testing.T) {
		entry.Info = image.Info{}
		entry.ExcludedReason = "just because"
		test(t, entry)
	})
}

// Check that existing entries, which are image.Info, will parse into
// the ImageEntry struct.
func TestImageInfoParsesAsEntry(t *testing.T) {
	ref, err := image.ParseRef("docker.io/fluxcd/flux:1.0.0")
	assert.NoError(t, err)
	info := image.Info{
		ID:        ref,
		CreatedAt: time.Now().UTC(), // to UTC since we unmarshal times in UTC
	}

	bytes, err := json.Marshal(info)
	assert.NoError(t, err)

	var entry2 ImageEntry
	assert.NoError(t, json.Unmarshal(bytes, &entry2))
	assert.Equal(t, info, entry2.Info)
}
