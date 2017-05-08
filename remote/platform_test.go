package remote

import (
	"testing"
)

// Just test that the mock does its job.
func TestPlatformMock(t *testing.T) {
	PlatformTestBattery(t, func(mock Platform) Platform { return mock })
}
