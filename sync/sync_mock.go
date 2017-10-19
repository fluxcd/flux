package sync

import (
	"fmt"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

type rsc struct {
	bytes []byte
	Kind  string
	Meta  struct {
		Namespace string
		Name      string
	}
}

type rscIgnorePolicy struct {
	bytes []byte
	Kind  string
	Meta  struct {
		Namespace string
		Name      string
	}
}

func (rs rsc) Source() string {
	return ""
}

func (rs rsc) Bytes() []byte {
	return []byte{}
}

func (rs rsc) ResourceID() flux.ResourceID {
	return flux.MakeResourceID(rs.Meta.Namespace, rs.Kind, rs.Meta.Name)
}

func (rs rsc) Policy() policy.Set {
	p := policy.Set{}
	return p
}

func (ri rscIgnorePolicy) Source() string {
	return ""
}

func (ri rscIgnorePolicy) Bytes() []byte {
	return []byte{}
}

func getBytes() []byte {
	return []byte{}
}

func (ri rscIgnorePolicy) ResourceID() flux.ResourceID {
	return flux.MakeResourceID(ri.Meta.Namespace, ri.Kind, ri.Meta.Name)
}

func (ri rscIgnorePolicy) Policy() policy.Set {
	p := policy.Set{}
	p[policy.Ignore] = "true"
	fmt.Printf("!!! policy = %+v\n", p)
	return p
}

func mockResourceWithoutIgnorePolicy(kind, namespace, name string) rsc {
	r := rsc{Kind: kind}
	r.Meta.Namespace = namespace
	r.Meta.Name = name
	return r
}

func mockResourceWithIgnorePolicy(kind, namespace, name string) rscIgnorePolicy {
	ri := rscIgnorePolicy{Kind: kind}
	ri.Meta.Namespace = namespace
	ri.Meta.Name = name
	return ri
}
