package resource

type Deployment struct {
	BaseObject
	Spec DeploymentSpec
}

type DeploymentSpec struct {
	Replicas int
	Template PodTemplate
}
