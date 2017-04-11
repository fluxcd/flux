package instance

import (
	"github.com/weaveworks/flux"
)

// IDMapper converts internal instance ids into external instance ids. Maybe
// this interface is overkill, we'll see what the users client looks like.
type IDMapper interface {
	ExternalInstanceID(flux.InstanceID) (string, error)
	InternalInstanceID(string) (flux.InstanceID, error)
}

// IdentityIDMapper is a noop ID Mapper which just returns the internal
// instance id. Useful in testing. Disastrous in prod.
var IdentityIDMapper identityIDMapper

type identityIDMapper struct{}

func (i identityIDMapper) ExternalInstanceID(internal flux.InstanceID) (string, error) {
	if internal == flux.DefaultInstanceID {
		return "", nil
	}
	return string(internal), nil
}

func (i identityIDMapper) InternalInstanceID(ext string) (flux.InstanceID, error) {
	if ext == "" {
		return flux.DefaultInstanceID, nil
	}
	return flux.InstanceID(ext), nil
}
