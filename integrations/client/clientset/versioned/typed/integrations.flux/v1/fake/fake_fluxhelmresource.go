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
	integrations_flux_v1 "github.com/weaveworks/flux/apis/integrations.flux/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeFluxHelmResources implements FluxHelmResourceInterface
type FakeFluxHelmResources struct {
	Fake *FakeIntegrationsV1
	ns   string
}

var fluxhelmresourcesResource = schema.GroupVersionResource{Group: "integrations.flux", Version: "v1", Resource: "fluxhelmresources"}

var fluxhelmresourcesKind = schema.GroupVersionKind{Group: "integrations.flux", Version: "v1", Kind: "FluxHelmResource"}

// Get takes name of the fluxHelmResource, and returns the corresponding fluxHelmResource object, and an error if there is any.
func (c *FakeFluxHelmResources) Get(name string, options v1.GetOptions) (result *integrations_flux_v1.FluxHelmResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(fluxhelmresourcesResource, c.ns, name), &integrations_flux_v1.FluxHelmResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*integrations_flux_v1.FluxHelmResource), err
}

// List takes label and field selectors, and returns the list of FluxHelmResources that match those selectors.
func (c *FakeFluxHelmResources) List(opts v1.ListOptions) (result *integrations_flux_v1.FluxHelmResourceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(fluxhelmresourcesResource, fluxhelmresourcesKind, c.ns, opts), &integrations_flux_v1.FluxHelmResourceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &integrations_flux_v1.FluxHelmResourceList{}
	for _, item := range obj.(*integrations_flux_v1.FluxHelmResourceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested fluxHelmResources.
func (c *FakeFluxHelmResources) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(fluxhelmresourcesResource, c.ns, opts))

}

// Create takes the representation of a fluxHelmResource and creates it.  Returns the server's representation of the fluxHelmResource, and an error, if there is any.
func (c *FakeFluxHelmResources) Create(fluxHelmResource *integrations_flux_v1.FluxHelmResource) (result *integrations_flux_v1.FluxHelmResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(fluxhelmresourcesResource, c.ns, fluxHelmResource), &integrations_flux_v1.FluxHelmResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*integrations_flux_v1.FluxHelmResource), err
}

// Update takes the representation of a fluxHelmResource and updates it. Returns the server's representation of the fluxHelmResource, and an error, if there is any.
func (c *FakeFluxHelmResources) Update(fluxHelmResource *integrations_flux_v1.FluxHelmResource) (result *integrations_flux_v1.FluxHelmResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(fluxhelmresourcesResource, c.ns, fluxHelmResource), &integrations_flux_v1.FluxHelmResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*integrations_flux_v1.FluxHelmResource), err
}

// Delete takes name of the fluxHelmResource and deletes it. Returns an error if one occurs.
func (c *FakeFluxHelmResources) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(fluxhelmresourcesResource, c.ns, name), &integrations_flux_v1.FluxHelmResource{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeFluxHelmResources) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(fluxhelmresourcesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &integrations_flux_v1.FluxHelmResourceList{})
	return err
}

// Patch applies the patch and returns the patched fluxHelmResource.
func (c *FakeFluxHelmResources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *integrations_flux_v1.FluxHelmResource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(fluxhelmresourcesResource, c.ns, name, data, subresources...), &integrations_flux_v1.FluxHelmResource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*integrations_flux_v1.FluxHelmResource), err
}
