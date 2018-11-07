package status

import (
	"encoding/json"

	"github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	v1beta1client "github.com/weaveworks/flux/integrations/client/clientset/versioned/typed/flux.weave.works/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

// We can't rely on having UpdateStatus, or strategic merge patching
// for custom resources. So we have to create an object which
// represents the merge path or JSON patch to apply.
func UpdateConditionsPatch(status *v1beta1.HelmReleaseStatus, updates ...v1beta1.HelmReleaseCondition) (types.PatchType, interface{}) {
	newConditions := make([]v1beta1.HelmReleaseCondition, len(status.Conditions))
	oldConditions := status.Conditions
	for i, c := range oldConditions {
		newConditions[i] = c
	}
updates:
	for _, up := range updates {
		for i, c := range oldConditions {
			if c.Type == up.Type {
				newConditions[i] = up
				continue updates
			}
		}
		newConditions = append(newConditions, up)
	}
	status.Conditions = newConditions
	return types.MergePatchType, map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": newConditions,
		},
	}
}

// UpdateConditions applies the updates to the HelmRelease given, and patches the resource in the cluster.
func UpdateConditions(client v1beta1client.HelmReleaseInterface, fhr *v1beta1.HelmRelease, updates ...v1beta1.HelmReleaseCondition) error {
	t, obj := UpdateConditionsPatch(&fhr.Status, updates...)
	bytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = client.Patch(fhr.Name, t, bytes)
	return err
}
