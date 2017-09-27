package resource

type Deployment struct {
	baseObject
	Spec DeploymentSpec
}

type DeploymentSpec struct {
	Replicas int
	Template PodTemplate
}
