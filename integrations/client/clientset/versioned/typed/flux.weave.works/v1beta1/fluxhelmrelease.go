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
package v1beta1

import (
	v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	scheme "github.com/weaveworks/flux/integrations/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// FluxHelmReleasesGetter has a method to return a FluxHelmReleaseInterface.
// A group's client should implement this interface.
type FluxHelmReleasesGetter interface {
	FluxHelmReleases(namespace string) FluxHelmReleaseInterface
}

// FluxHelmReleaseInterface has methods to work with FluxHelmRelease resources.
type FluxHelmReleaseInterface interface {
	Create(*v1beta1.FluxHelmRelease) (*v1beta1.FluxHelmRelease, error)
	Update(*v1beta1.FluxHelmRelease) (*v1beta1.FluxHelmRelease, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1beta1.FluxHelmRelease, error)
	List(opts v1.ListOptions) (*v1beta1.FluxHelmReleaseList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.FluxHelmRelease, err error)
	FluxHelmReleaseExpansion
}

// fluxHelmReleases implements FluxHelmReleaseInterface
type fluxHelmReleases struct {
	client rest.Interface
	ns     string
}

// newFluxHelmReleases returns a FluxHelmReleases
func newFluxHelmReleases(c *FluxV1beta1Client, namespace string) *fluxHelmReleases {
	return &fluxHelmReleases{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the fluxHelmRelease, and returns the corresponding fluxHelmRelease object, and an error if there is any.
func (c *fluxHelmReleases) Get(name string, options v1.GetOptions) (result *v1beta1.FluxHelmRelease, err error) {
	result = &v1beta1.FluxHelmRelease{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of FluxHelmReleases that match those selectors.
func (c *fluxHelmReleases) List(opts v1.ListOptions) (result *v1beta1.FluxHelmReleaseList, err error) {
	result = &v1beta1.FluxHelmReleaseList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested fluxHelmReleases.
func (c *fluxHelmReleases) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a fluxHelmRelease and creates it.  Returns the server's representation of the fluxHelmRelease, and an error, if there is any.
func (c *fluxHelmReleases) Create(fluxHelmRelease *v1beta1.FluxHelmRelease) (result *v1beta1.FluxHelmRelease, err error) {
	result = &v1beta1.FluxHelmRelease{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		Body(fluxHelmRelease).
		Do().
		Into(result)
	return
}

// Update takes the representation of a fluxHelmRelease and updates it. Returns the server's representation of the fluxHelmRelease, and an error, if there is any.
func (c *fluxHelmReleases) Update(fluxHelmRelease *v1beta1.FluxHelmRelease) (result *v1beta1.FluxHelmRelease, err error) {
	result = &v1beta1.FluxHelmRelease{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		Name(fluxHelmRelease.Name).
		Body(fluxHelmRelease).
		Do().
		Into(result)
	return
}

// Delete takes name of the fluxHelmRelease and deletes it. Returns an error if one occurs.
func (c *fluxHelmReleases) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *fluxHelmReleases) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched fluxHelmRelease.
func (c *fluxHelmReleases) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1beta1.FluxHelmRelease, err error) {
	result = &v1beta1.FluxHelmRelease{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("fluxhelmreleases").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
