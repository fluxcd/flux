package resource

type List struct {
	baseObject
	Items []KubeManifest
}
