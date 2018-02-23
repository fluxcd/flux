/*
Copyright 2018 The Kubernetes Authors.

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

package v1

import (
	v1 "github.com/weaveworks/flux/apis/integrations.flux/v1"
	scheme "github.com/weaveworks/flux/integrations/client/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// FluxHelmResourcesGetter has a method to return a FluxHelmResourceInterface.
// A group's client should implement this interface.
type FluxHelmResourcesGetter interface {
	FluxHelmResources(namespace string) FluxHelmResourceInterface
}

// FluxHelmResourceInterface has methods to work with FluxHelmResource resources.
type FluxHelmResourceInterface interface {
	Create(*v1.FluxHelmResource) (*v1.FluxHelmResource, error)
	Update(*v1.FluxHelmResource) (*v1.FluxHelmResource, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.FluxHelmResource, error)
	List(opts meta_v1.ListOptions) (*v1.FluxHelmResourceList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.FluxHelmResource, err error)
	FluxHelmResourceExpansion
}

// fluxHelmResources implements FluxHelmResourceInterface
type fluxHelmResources struct {
	client rest.Interface
	ns     string
}

// newFluxHelmResources returns a FluxHelmResources
func newFluxHelmResources(c *IntegrationsV1Client, namespace string) *fluxHelmResources {
	return &fluxHelmResources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the fluxHelmResource, and returns the corresponding fluxHelmResource object, and an error if there is any.
func (c *fluxHelmResources) Get(name string, options meta_v1.GetOptions) (result *v1.FluxHelmResource, err error) {
	result = &v1.FluxHelmResource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of FluxHelmResources that match those selectors.
func (c *fluxHelmResources) List(opts meta_v1.ListOptions) (result *v1.FluxHelmResourceList, err error) {
	result = &v1.FluxHelmResourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested fluxHelmResources.
func (c *fluxHelmResources) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a fluxHelmResource and creates it.  Returns the server's representation of the fluxHelmResource, and an error, if there is any.
func (c *fluxHelmResources) Create(fluxHelmResource *v1.FluxHelmResource) (result *v1.FluxHelmResource, err error) {
	result = &v1.FluxHelmResource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		Body(fluxHelmResource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a fluxHelmResource and updates it. Returns the server's representation of the fluxHelmResource, and an error, if there is any.
func (c *fluxHelmResources) Update(fluxHelmResource *v1.FluxHelmResource) (result *v1.FluxHelmResource, err error) {
	result = &v1.FluxHelmResource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		Name(fluxHelmResource.Name).
		Body(fluxHelmResource).
		Do().
		Into(result)
	return
}

// Delete takes name of the fluxHelmResource and deletes it. Returns an error if one occurs.
func (c *fluxHelmResources) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *fluxHelmResources) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("fluxhelmresources").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched fluxHelmResource.
func (c *fluxHelmResources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.FluxHelmResource, err error) {
	result = &v1.FluxHelmResource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("fluxhelmresources").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
