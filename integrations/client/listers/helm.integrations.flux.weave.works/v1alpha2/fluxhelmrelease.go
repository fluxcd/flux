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
package v1alpha2

import (
	v1alpha2 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// FluxHelmReleaseLister helps list FluxHelmReleases.
type FluxHelmReleaseLister interface {
	// List lists all FluxHelmReleases in the indexer.
	List(selector labels.Selector) (ret []*v1alpha2.FluxHelmRelease, err error)
	// FluxHelmReleases returns an object that can list and get FluxHelmReleases.
	FluxHelmReleases(namespace string) FluxHelmReleaseNamespaceLister
	FluxHelmReleaseListerExpansion
}

// fluxHelmReleaseLister implements the FluxHelmReleaseLister interface.
type fluxHelmReleaseLister struct {
	indexer cache.Indexer
}

// NewFluxHelmReleaseLister returns a new FluxHelmReleaseLister.
func NewFluxHelmReleaseLister(indexer cache.Indexer) FluxHelmReleaseLister {
	return &fluxHelmReleaseLister{indexer: indexer}
}

// List lists all FluxHelmReleases in the indexer.
func (s *fluxHelmReleaseLister) List(selector labels.Selector) (ret []*v1alpha2.FluxHelmRelease, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.FluxHelmRelease))
	})
	return ret, err
}

// FluxHelmReleases returns an object that can list and get FluxHelmReleases.
func (s *fluxHelmReleaseLister) FluxHelmReleases(namespace string) FluxHelmReleaseNamespaceLister {
	return fluxHelmReleaseNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// FluxHelmReleaseNamespaceLister helps list and get FluxHelmReleases.
type FluxHelmReleaseNamespaceLister interface {
	// List lists all FluxHelmReleases in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha2.FluxHelmRelease, err error)
	// Get retrieves the FluxHelmRelease from the indexer for a given namespace and name.
	Get(name string) (*v1alpha2.FluxHelmRelease, error)
	FluxHelmReleaseNamespaceListerExpansion
}

// fluxHelmReleaseNamespaceLister implements the FluxHelmReleaseNamespaceLister
// interface.
type fluxHelmReleaseNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all FluxHelmReleases in the indexer for a given namespace.
func (s fluxHelmReleaseNamespaceLister) List(selector labels.Selector) (ret []*v1alpha2.FluxHelmRelease, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.FluxHelmRelease))
	})
	return ret, err
}

// Get retrieves the FluxHelmRelease from the indexer for a given namespace and name.
func (s fluxHelmReleaseNamespaceLister) Get(name string) (*v1alpha2.FluxHelmRelease, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha2.Resource("fluxhelmrelease"), name)
	}
	return obj.(*v1alpha2.FluxHelmRelease), nil
}
