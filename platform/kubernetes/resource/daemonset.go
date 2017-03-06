package resource

type DaemonSet struct {
	baseObject
	Spec DaemonSetSpec
}

type DaemonSetSpec struct {
	Selector map[string]string
	Template PodTemplate
}
