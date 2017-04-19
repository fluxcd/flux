package resource

// Types that daemonsets, deployments, and other things have in
// common.

type ObjectMeta struct {
	Labels      map[string]string
	Annotations map[string]string
}

type PodTemplate struct {
	Metadata ObjectMeta
	Spec     PodSpec
}

type PodSpec struct {
	ImagePullSecrets []struct{ Name string }
	Volumes          []Volume
	Containers       []ContainerSpec
}

type Volume struct {
	Name   string
	Secret struct {
		SecretName string
	}
}

type ContainerSpec struct {
	Name  string
	Image string
	Args  Args
	Ports []ContainerPort
	Env   Env
}

type Args []string

type ContainerPort struct {
	ContainerPort int
	Name          string
}

type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

// Env is a bag of Name, Value pairs that are treated somewhat like a
// map.
type Env []EnvEntry

type EnvEntry struct {
	Name, Value string
}
