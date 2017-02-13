package release

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/platform"
)

// Operations on instances (or instance.* types) that we need for
// releasing

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
func CollectAvailableImages(inst *instance.Instance, updateable []*ServiceUpdate) (instance.ImageMap, error) {
	var servicesToCheck []platform.Service
	for _, update := range updateable {
		servicesToCheck = append(servicesToCheck, update.Service)
	}
	return inst.CollectAvailableImages(servicesToCheck)
}

// applyChanges effects the calculated changes on the platform.
func applyChanges(inst *instance.Instance, updates []*ServiceUpdate, results flux.ReleaseResult) error {
	// Collect definitions for each service release.
	var defs []platform.ServiceDefinition
	// If we're regrading our own image, we want to do that
	// last, and "asynchronously" (meaning we probably won't
	// see the reply).
	var asyncDefs []platform.ServiceDefinition

	for _, update := range updates {
		namespace, serviceName := update.ServiceID.Components()
		switch serviceName {
		case FluxServiceName, FluxDaemonName:
			inst.LogEvent(namespace, serviceName, "Starting. (no result expected)")
			asyncDefs = append(asyncDefs, platform.ServiceDefinition{
				ServiceID:     update.ServiceID,
				NewDefinition: update.ManifestBytes,
				Async:         true,
			})
		default:
			inst.LogEvent(namespace, serviceName, "Starting")
			defs = append(defs, platform.ServiceDefinition{
				ServiceID:     update.ServiceID,
				NewDefinition: update.ManifestBytes,
			})
		}
		// Mark as successful, until we have an answer
		result := results[update.ServiceID]
		results[update.ServiceID] = flux.ServiceResult{
			Status:       flux.ReleaseStatusSuccess,
			Error:        result.Error,
			PerContainer: result.PerContainer,
		}
	}

	transactionErr := inst.PlatformApply(defs)
	if transactionErr != nil {
		switch err := transactionErr.(type) {
		case platform.ApplyError:
			for id, applyErr := range err {
				results[id] = flux.ServiceResult{
					Status: flux.ReleaseStatusFailed,
					Error:  applyErr.Error(),
				}
			}
		default:
			for _, update := range updates {
				results[update.ServiceID] = flux.ServiceResult{
					Status: flux.ReleaseStatusUnknown,
					Error:  transactionErr.Error(),
				}
			}
			// assume everything that was planned failed, if there
			// was a coverall error. Note that this _includes_ the
			// async releases, since if there's a problem, we don't attempt
			// them.
			return transactionErr
		}
	}
	// Lastly, services for which we don't expect a result
	// (i.e., ourselves). This will kick off the release in
	// the daemon, which will cause Kubernetes to restart the
	// service. In the meantime, however, we will have
	// finished recording what happened, as part of a graceful
	// shutdown. So the only thing that goes missing is the
	// result from this release call.
	if len(asyncDefs) > 0 {
		inst.PlatformApply(asyncDefs)
	}

	return transactionErr
}
