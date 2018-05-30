package resource

import (
	"github.com/weaveworks/flux/resource"
)

type List struct {
	baseObject
	Items []resource.Resource
}
