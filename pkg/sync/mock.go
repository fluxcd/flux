package sync

import (
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
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
	rsc
}

func (rs rsc) Source() string {
	return ""
}

func (rs rsc) Bytes() []byte {
	return []byte{}
}

func (rs rsc) ResourceID() resource.ID {
	return resource.MakeID(rs.Meta.Namespace, rs.Kind, rs.Meta.Name)
}

func (rs rsc) Policy() policy.Set {
	p := policy.Set{}
	return p
}

func (ri rscIgnorePolicy) Policy() policy.Set {
	p := policy.Set{}
	p[policy.Ignore] = "true"
	return p
}

func mockResourceWithoutIgnorePolicy(kind, namespace, name string) rsc {
	r := rsc{Kind: kind}
	r.Meta.Namespace = namespace
	r.Meta.Name = name
	return r
}

func mockResourceWithIgnorePolicy(kind, namespace, name string) rscIgnorePolicy {
	ri := rscIgnorePolicy{rsc{Kind: kind}}
	ri.Meta.Namespace = namespace
	ri.Meta.Name = name
	return ri
}
