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
	Settings flux.InstanceConfig              `json:"settings"`
}

type NamedConfig struct {
	ID     flux.InstanceID
	Config Config
}

func MakeConfig() Config {
	return Config{
		Services: map[flux.ServiceID]ServiceConfig{},
	}
}

type UpdateFunc func(config Config) (Config, error)

type DB interface {
	UpdateConfig(instance flux.InstanceID, update UpdateFunc) error
	GetConfig(instance flux.InstanceID) (Config, error)
	All() ([]NamedConfig, error)
}

type Configurer interface {
	Get() (Config, error)
	Update(UpdateFunc) error
}
