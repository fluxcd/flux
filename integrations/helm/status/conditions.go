package status

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	v1beta1client "github.com/weaveworks/flux/integrations/client/clientset/versioned/typed/flux.weave.works/v1beta1"
)


// NewCondition creates a new HelmReleaseCondition.
func NewCondition(conditionType v1beta1.HelmReleaseConditionType, status v1.ConditionStatus, reason, message string) v1beta1.HelmReleaseCondition {
	return v1beta1.HelmReleaseCondition{
		Type: conditionType,
		Status: status,
		LastUpdateTime: metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason: reason,
		Message: message,
	}
}

// SetCondition updates the HelmRelease to include the given condition.
func SetCondition(client v1beta1client.HelmReleaseInterface, hr v1beta1.HelmRelease,
	condition v1beta1.HelmReleaseCondition) error {

	cHr, err := client.Get(hr.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	currCondition := GetCondition(cHr.Status, condition.Type)
	if currCondition != nil && currCondition.Status == condition.Status {
		condition.LastTransitionTime = currCondition.LastTransitionTime
	}

	newConditions := filterOutCondition(cHr.Status.Conditions, condition.Type)
	cHr.Status.Conditions = append(newConditions, condition)

	_, err = client.UpdateStatus(cHr)
	return err
}

// GetCondition returns the condition with the given type.
func GetCondition(status v1beta1.HelmReleaseStatus, conditionType v1beta1.HelmReleaseConditionType) *v1beta1.HelmReleaseCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == conditionType {
			return &c
		}
	}
	return nil
}

// filterOutCondition returns a new slice of conditions without the
// conditions of the given type.
func filterOutCondition(conditions []v1beta1.HelmReleaseCondition, conditionType v1beta1.HelmReleaseConditionType) []v1beta1.HelmReleaseCondition {
	var newConditions []v1beta1.HelmReleaseCondition
	for _, c := range conditions {
		if c.Type == conditionType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}
