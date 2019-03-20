package cluster

import (
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/resource"
)

// An Annotation change indicates how an annotation should be changed
type AnnotationChange struct {
	AnnotationKey string
	// AnnotationValue is the value to set the annotation to, nil indicates delete
	AnnotationValue *string
}

type PolicyTranslator interface {
	// GetAnnotationChangesForPolicyUpdate translates a policy update into annotation updates
	GetAnnotationChangesForPolicyUpdate(workload resource.Workload, update policy.Update) ([]AnnotationChange, error)
}
