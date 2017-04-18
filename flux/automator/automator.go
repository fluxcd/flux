// Package automator implements automation behavior.
package automator

import (
	"github.com/weaveworks/fluxy/flux"
)

// Automator enables and disables automation for a specific service.
type Automator interface {
	Automate(flux.ServiceID) error
	Deautomate(flux.ServiceID) error
}
