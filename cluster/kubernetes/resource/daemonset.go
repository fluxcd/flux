package resource

type DaemonSet struct {
	BaseObject
	Spec DaemonSetSpec
}

type DaemonSetSpec struct {
	Template PodTemplate
}
