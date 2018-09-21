/*
Copyright 2018 Weaveworks Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package fake

import (
	v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeFluxHelmReleases implements FluxHelmReleaseInterface
type FakeFluxHelmReleases struct {
	Fake *FakeFluxV1beta1
	ns   string
}

var fluxhelmreleasesResource = schema.GroupVersionResource{Group: "flux.weave.works", Version: "v1beta1", Resource: "fluxhelmreleases"}

var fluxhelmreleasesKind = schema.GroupVersionKind{Group: "flux.weave.works", Version: "v1beta1", Kind: "FluxHelmRelease"}

// Get takes name of the fluxHelmRelease, and returns the corresponding fluxHelmRelease object, and an error if there is any.
func (c *FakeFluxHelmReleases) Get(name string, options v1.GetOptions) (result *v1beta1.FluxHelmRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(fluxhelmreleasesResource, c.ns, name), &v1beta1.FluxHelmRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.FluxHelmRelease), err
}

// List takes label and field selectors, and returns the list of FluxHelmReleases that match those selectors.
func (c *FakeFluxHelmReleases) List(opts v1.ListOptions) (result *v1beta1.FluxHelmReleaseList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(fluxhelmreleasesResource, fluxhelmreleasesKind, c.ns, opts), &v1beta1.FluxHelmReleaseList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.FluxHelmReleaseList{ListMeta: obj.(*v1beta1.FluxHelmReleaseList).ListMeta}
	for _, item := range obj.(*v1beta1.FluxHelmReleaseList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested fluxHelmReleases.
func (c *FakeFluxHelmReleases) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(fluxhelmreleasesResource, c.ns, opts))

}

// Create takes the representation of a fluxHelmRelease and creates it.  Returns the server's representation of the fluxHelmRelease, and an error, if there is any.
func (c *FakeFluxHelmReleases) Create(fluxHelmRelease *v1beta1.FluxHelmRelease) (result *v1beta1.FluxHelmRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(fluxhelmreleasesResource, c.ns, fluxHelmRelease), &v1beta1.FluxHelmRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.FluxHelmRelease), err
}

// Update takes the representation of a fluxHelmRelease and updates it. Returns the server's representation of the fluxHelmRelease, and an error, if there is any.
func (c *FakeFluxHelmReleases) Update(fluxHelmRelease *v1beta1.FluxHelmRelease) (result *v1beta1.FluxHelmRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(fluxhelmreleasesResource, c.ns, fluxHelmRelease), &v1beta1.FluxHelmRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.FluxHelmRelease), err
}

// Delete takes name of the fluxHelmRelease and deletes it. Returns an error if one occurs.
func (c *FakeFluxHelmReleases) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(fluxhelmreleasesResource, c.ns, name), &v1beta1.FluxHelmRelease{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeFluxHelmReleases) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(fluxhelmreleasesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1beta1.FluxHelmReleaseList{})
	return err
}

// Patch applies the patch and returns the patched fluxHelmRelease.
func (c *FakeFluxHelmReleases) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.FluxHelmRelease, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(fluxhelmreleasesResource, c.ns, name, data, subresources...), &v1beta1.FluxHelmRelease{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.FluxHelmRelease), err
}
