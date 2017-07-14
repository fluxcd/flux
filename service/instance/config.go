package instance

import (
	"time"

	"github.com/weaveworks/flux/service"
)

type Connection struct {
	Last      time.Time `json:"last"`
	Connected bool      `json:"connected"`
}

type Config struct {
	Settings   service.InstanceConfig `json:"settings"`
	Connection Connection             `json:"connection"`
}

type UpdateFunc func(config Config) (Config, error)

type DB interface {
	UpdateConfig(instance service.InstanceID, update UpdateFunc) error
	GetConfig(instance service.InstanceID) (Config, error)
}

type Configurer interface {
	Get() (Config, error)
	Update(UpdateFunc) error
}

type configurer struct {
	instance service.InstanceID
	db       DB
}

func (c configurer) Get() (Config, error) {
	return c.db.GetConfig(c.instance)
}

func (c configurer) Update(update UpdateFunc) error {
	return c.db.UpdateConfig(c.instance, update)
}
