package resource

type ReplicationController struct {
	baseObject
	Spec ReplicationControllerSpec
}

type ReplicationControllerSpec struct {
	Replicas int
	Template PodTemplate
}
