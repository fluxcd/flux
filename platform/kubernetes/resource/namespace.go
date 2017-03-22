package resource

import (
	"github.com/weaveworks/flux/diff"
)

type Namespace struct {
	baseObject
}

func (ns *Namespace) ID() diff.ObjectID {
	return diff.ObjectID{
		Kind:      ns.Kind,
		Name:      ns.Meta.Name,
		Namespace: "",
	}
}
