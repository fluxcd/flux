package platform

import (
	"errors"

	"github.com/weaveworks/fluxy"
)

// StandaloneConnecter supplies a single Platform given the
// pre-arranged instance ID. This is to support configurations in
// which the service deals only with its local cluster.
type StandaloneConnecter struct {
	Instance     flux.InstanceID
	LocalCluster Platform
}

func (c *StandaloneConnecter) Connect(id flux.InstanceID) (Platform, error) {
	if id == c.Instance {
		return c.LocalCluster, nil
	}
	return nil, errors.New("standalone connecter cannot connect to instance other than " + string(c.Instance))
}
