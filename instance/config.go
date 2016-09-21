package instance

import (
	"github.com/weaveworks/fluxy"
)

type ServiceConfig struct {
	Automated bool `json:"automation"`
}

type InstanceConfig struct {
	Services map[flux.ServiceID]ServiceConfig `json:"services"`
}

func MakeConfig() InstanceConfig {
	return InstanceConfig{
		Services: map[flux.ServiceID]ServiceConfig{},
	}
}

type UpdateFunc func(config InstanceConfig) (InstanceConfig, error)

type DB interface {
	Update(instance flux.InstanceID, update UpdateFunc) error
	Get(instance flux.InstanceID) (InstanceConfig, error)
}
