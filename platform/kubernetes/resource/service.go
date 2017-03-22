package resource

// For reference:
// https://github.com/kubernetes/client-go/blob/master/pkg/api/v1/types.go#L2641

type Service struct {
	baseObject
	Spec ServiceSpec `yaml:"spec"`
}

type ServiceSpec struct {
	Type     string            `yaml:"type"`
	Ports    []ServicePort     `yaml:"ports"`
	Selector map[string]string `yaml:"selector"`
}

type ServicePort struct {
	Name       string `yaml:"name"`
	Protocol   string `yaml:"protocol"`
	Port       int32  `yaml:"port"`
	TargetPort string `yaml:"targetPort"`
	NodePort   int32  `yaml:"nodePort"`
}
