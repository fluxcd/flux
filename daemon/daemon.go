package daemon

import (
	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/platform"
)

type Daemon interface {
	Daemon(flux.InstanceID, platform.Platform) error
}
