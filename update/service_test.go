package update_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/update"
)

func TestControllerUpdate_Filter(t *testing.T) {
	include, _ := flux.ParseResourceID("ns:kind/include")
	exclude, _ := flux.ParseResourceID("ns:kind/exclude")
	locked, _ := flux.ParseResourceID("ns:kind/locked")
	unknown, _ := flux.ParseResourceID("ns:kind/unknown")
	fs := []update.ControllerFilter{
		&update.IncludeFilter{IDs: []flux.ResourceID{include, locked}},
		&update.ExcludeFilter{IDs: []flux.ResourceID{exclude}},
		&update.LockedFilter{IDs: []flux.ResourceID{locked}},
	}

	{ // include
		u := &update.ControllerUpdate{ResourceID: include}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{}, res)
	}
	{ // exclude
		u := &update.ControllerUpdate{ResourceID: exclude}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{
			Status: "ignored",
			Error: "not included",
		}, res)
	}
	{ // not mentioned
		u := &update.ControllerUpdate{ResourceID: unknown}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{
			Status: "ignored",
			Error: "not included",
		}, res)
	}
	{ // locked
		u := &update.ControllerUpdate{ResourceID: locked}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{
			Status: "skipped",
			Error: "locked",
		}, res)

	}
}

func TestControllerUpdate_NamespacesFilter(t *testing.T) {
	id, _ := flux.ParseResourceID("ns:kind/one")

	{ // single namespace filter
		nsf := &update.NamespacesFilter{}
		nsf.Add("ns", "kind")
		fs := []update.ControllerFilter{nsf}

		u := &update.ControllerUpdate{ResourceID: id}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{}, res)
	}
	{ // multi namespace filter
		nsf := &update.NamespacesFilter{}
		nsf.Add("ns", "kind")
		nsf.Add("ns2", "kind")
		fs := []update.ControllerFilter{nsf}

		u := &update.ControllerUpdate{ResourceID: id}
		res := u.Filter(fs...)
		assert.Equal(t, update.ControllerResult{}, res)
	}
}

func TestControllerUpdate_IncludeFilter(t *testing.T) {
	{ // match
		id, _ := flux.ParseResourceID("ns:kind/name")
		u := &update.ControllerUpdate{ResourceID: id}
		res := u.Filter(&update.IncludeFilter{IDs: []flux.ResourceID{id}})
		assert.Empty(t, res.Error)
	}
	{ // no match
		id, _ := flux.ParseResourceID("ns:kind/name")
		filtered := []flux.ResourceID{
			flux.MakeResourceID("nsX", "kind", "name"),
			flux.MakeResourceID("ns", "kindX", "name"),
			flux.MakeResourceID("ns", "kind", "nameX"),
		}
		u := &update.ControllerUpdate{ResourceID: id}
		res := u.Filter(&update.IncludeFilter{IDs: filtered})
		assert.NotEmpty(t, res.Error)
	}
}
