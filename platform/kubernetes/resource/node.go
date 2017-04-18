package resource

import (
	"github.com/weaveworks/flux/diff"
)

type Node struct {
	baseObject
	Metadata ObjectMeta
}

func (ns *Node) ID() diff.ObjectID {
	return diff.ObjectID{
		Kind:      ns.Kind,
		Name:      ns.Meta.Name,
		Namespace: "",
	}
}
