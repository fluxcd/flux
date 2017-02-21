package instance

import (
	"github.com/weaveworks/flux"
)

type MockInstancer struct {
	Instance *Instance
	Error    error
}

func (m *MockInstancer) Get(_ flux.InstanceID) (*Instance, error) {
	return m.Instance, m.Error
}

type MockConfigurer struct {
	Config Config
	Error  error
}

func (c *MockConfigurer) Get() (Config, error) {
	return c.Config, c.Error
}

func (c *MockConfigurer) Update(up UpdateFunc) error {
	newConfig, err := up(c.Config)
	if err == nil {
		c.Config = newConfig
	}
	return err
}
