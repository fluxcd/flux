package instance

import (
	"github.com/weaveworks/fluxy"
)

type ServiceConfig struct {
	Automated bool `json:"automation"`
	Locked    bool `json:"locked"`
}

func (c ServiceConfig) Policy() flux.Policy {
	if c.Locked {
		return flux.PolicyLocked
	}
	if c.Automated {
		return flux.PolicyAutomated
	}
	return flux.PolicyNone
}

type Config struct {
	Services map[flux.ServiceID]ServiceConfig `json:"services"`
}

func MakeConfig() Config {
	return Config{
		Services: map[flux.ServiceID]ServiceConfig{},
	}
}

type UpdateFunc func(config Config) (Config, error)

type DB interface {
	Update(instance flux.InstanceID, update UpdateFunc) error
	Get(instance flux.InstanceID) (Config, error)
}
