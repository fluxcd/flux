package resource

type DaemonSet struct {
	baseObject
	Spec DaemonSetSpec
}

type DaemonSetSpec struct {
	Template PodTemplate
}
