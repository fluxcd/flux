package resource

import (
	"fmt"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

const (
	PolicyPrefix = "flux.weave.works/"
)

// -- unmarshaling code for specific object and field types

// struct to embed in objects, to provide default implementation
type baseDockerObject struct {
	source    string
	bytes     []byte
	namespace string
	Version   string                 `yaml:"version"`
	Services  map[string]interface{} `yaml:"services"`
}

func (o baseDockerObject) ResourceID() string {
	var key string
	ns := o.namespace
	if ns == "" {
		ns = "default"
	}
	for c, _ := range o.Services {
		key = c
	}
	return fmt.Sprintf("%s/%s", ns, key)
}

// It's useful for comparisons in tests to be able to remove the
// record of bytes
func (o *baseDockerObject) debyte() {
	o.bytes = nil
}

// ServiceIDs reports the services that depend on this resource.
func (o baseDockerObject) ServiceIDs(all map[string]resource.Resource) []flux.ServiceID {
	return nil
}

func (o baseDockerObject) Policy() policy.Set {
	set := policy.Set{}
	return set
}

func (o baseDockerObject) Source() string {
	return o.source
}

func (o baseDockerObject) Bytes() []byte {
	return o.bytes
}
