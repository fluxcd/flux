package release

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"
)

func LockedServices(config instance.Config) flux.ServiceIDSet {
	ids := []flux.ServiceID{}
	for id, s := range config.Services {
		if s.Locked {
			ids = append(ids, id)
		}
	}
	idSet := flux.ServiceIDSet{}
	idSet.Add(ids)
	return idSet
}

// CollectAvailableImages is a convenient shim to
// `instance.CollectAvailableImages`.
func CollectAvailableImages(registry registry.Registry, updateable []*ServiceUpdate) (platform.ImageMap, error) {
	var servicesToCheck []platform.Service
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Service)
	}
	return platform.CollectAvailableImages(registry, servicesToCheck)
}
