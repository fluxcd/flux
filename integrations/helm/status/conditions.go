package status

import (
	"github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	v1beta1client "github.com/weaveworks/flux/integrations/client/clientset/versioned/typed/flux.weave.works/v1beta1"
)

// We can't rely on having UpdateStatus, or strategic merge patching
// for custom resources. So we have to create an object which
// represents the merge path or JSON patch to apply.
func UpdateConditionsPatch(status *v1beta1.HelmReleaseStatus, updates ...v1beta1.HelmReleaseCondition) {
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
}

// UpdateConditions applies the updates to the HelmRelease given, and
// updates the resource in the cluster.
func UpdateConditions(client v1beta1client.HelmReleaseInterface, fhr *v1beta1.HelmRelease, updates ...v1beta1.HelmReleaseCondition) error {
	fhrCopy := fhr.DeepCopy()

	UpdateConditionsPatch(&fhrCopy.Status, updates...)
	_, err := client.UpdateStatus(fhrCopy)

	return err
}
