package resource

type ConfigMap struct {
	baseObject
	Data map[string]string
}
