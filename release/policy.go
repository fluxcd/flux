package release

import (
	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/update"
)

func ApplyPolicy(rc *ReleaseContext, p update.Policy, log log.Logger) (update.Result, error) {
	result := update.Result{}
	for serviceID, u := range p {
		err := cluster.UpdateManifest(rc.Manifests(), rc.repo.ManifestDir(), serviceID, func(def []byte) ([]byte, error) {
			newDef, err := rc.Manifests().UpdatePolicies(def, u.Add, u.Remove)
			if err != nil {
				result[serviceID] = update.ControllerResult{
					Status: update.ReleaseStatusFailed,
					Error:  err.Error(),
				}
				return nil, err
			}
			if string(newDef) == string(def) {
				result[serviceID] = update.ControllerResult{
					Status: update.ReleaseStatusSkipped,
				}
			} else {
				result[serviceID] = update.ControllerResult{
					Status: update.ReleaseStatusSuccess,
				}
			}
			return newDef, nil
		})
		switch err {
		case cluster.ErrNoResourceFilesFoundForService, cluster.ErrMultipleResourceFilesFoundForService:
			result[serviceID] = update.ControllerResult{
				Status: update.ReleaseStatusFailed,
				Error:  err.Error(),
			}
		case nil:
			// continue
		default:
			return result, err
		}
	}

	return result, nil
}
