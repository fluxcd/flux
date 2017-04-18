package orgmap

import (
	"github.com/pkg/errors"
	"github.com/weaveworks/fluxy/flux/fluxd"
)

// Demuxer maps an orgID to a fluxd reference.
// Typically implemented by the shared DB.
type Demuxer interface {
	Demux(orgID string) (ref string, err error)
}

// Connector maps a fluxd reference to a working fluxd.
// Typically implemented by a pipes service connector.
type Connector interface {
	Connect(ref string) (fluxd.Service, error)
}

// Mapper maps org IDs to fluxds.
type Mapper struct {
	d Demuxer
	c Connector
}

// NewMapper returns a usable mapper.
// Package db provides a Demuxer implementation.
// Package pipe provides a Connector implementation.
func NewMapper(d Demuxer, c Connector) *Mapper {
	return &Mapper{
		d: d,
		c: c,
	}
}

// Map maps org ID to a fluxd.
func (m *Mapper) Map(orgID string) (fluxd.Service, error) {
	ref, err := m.d.Demux(orgID)
	if err != nil {
		return nil, errors.Wrapf(err, "demuxing %s", orgID)
	}
	service, err := m.c.Connect(ref)
	if err != nil {
		return nil, errors.Wrapf(err, "connecting to %s via %s", orgID, ref)
	}
	return service, nil
}
