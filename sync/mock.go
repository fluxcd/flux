package sync

import (
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
	rsc
}

type mockRes struct {
	r  rsc
	ri rscIgnorePolicy
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

func (ri rscIgnorePolicy) Policy() policy.Set {
	p := policy.Set{}
	p[policy.Ignore] = "true"
	return p
}

func getBytes() []byte {
	return []byte{}
}

func mockResource(kind, namespace, name string) rsc {
	r := rsc{Kind: kind}
	r.Meta.Namespace = namespace
	r.Meta.Name = name
	return r
}

func mockResourceWithoutIgnorePolicy(kind, namespace, name string) mockRes {
	r := rsc{Kind: kind}
	r.Meta.Namespace = namespace
	r.Meta.Name = name
	return mockRes{r: r}
}

func mockResourceWithIgnorePolicy(kind, namespace, name string) mockRes {
	r := rsc{Kind: kind}
	r.Meta.Namespace = namespace
	r.Meta.Name = name
	ri := rscIgnorePolicy{rsc: r}
	return mockRes{ri: ri}
}
