package remote

import (
	"testing"

	"github.com/weaveworks/flux/api"
)

// Just test that the mock does its job.
func TestPlatformMock(t *testing.T) {
	PlatformTestBattery(t, func(mock api.Server) api.Server { return mock })
}
