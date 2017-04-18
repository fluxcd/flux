package pipe

import (
	"errors"

	"github.com/weaveworks/fluxy/flux/fluxd"
)

// Pipe provides a way to talk to connected fluxds.
type Pipe struct{}

// Connect creates a fluxd.Service to the connected fluxd identified by ref.
// Connect implements the orgmap.Connector interface.
func (p *Pipe) Connect(ref string) (fluxd.Service, error) {
	return nil, errors.New("not implemented")
}
