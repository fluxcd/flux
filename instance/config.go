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

type DB interface {
	Set(instance flux.InstanceID, config InstanceConfig) error
	Get(instance flux.InstanceID) (InstanceConfig, error)
}
