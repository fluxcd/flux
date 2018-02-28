package resource

type List struct {
	Kind  string       `yaml:"kind"`
	Items []BaseObject `yaml:"items"`
}
