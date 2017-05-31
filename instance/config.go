package instance

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

type ServiceConfig struct {
	Automated bool `json:"automation"`
	Locked    bool `json:"locked"`
}

func (c ServiceConfig) Policy() policy.Policy {
	if c.Locked {
		return policy.Locked
	}
	if c.Automated {
		return policy.Automated
	}
	return policy.None
}

type Config struct {
	Services map[flux.ServiceID]ServiceConfig `json:"services"`
	Settings flux.UnsafeInstanceConfig        `json:"settings"`
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

type configurer struct {
	instance flux.InstanceID
	db       DB
}

func (c configurer) Get() (Config, error) {
	return c.db.GetConfig(c.instance)
}

func (c configurer) Update(update UpdateFunc) error {
	return c.db.UpdateConfig(c.instance, update)
}
