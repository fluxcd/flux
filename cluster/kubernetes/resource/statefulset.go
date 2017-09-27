package resource

type StatefulSet struct {
	baseObject
	Spec StatefulSetSpec
}

type StatefulSetSpec struct {
	Replicas int
	Template PodTemplate
}
