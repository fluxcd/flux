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
	helm_integrations_flux_weave_works_v1alpha2 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	versioned "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	internalinterfaces "github.com/weaveworks/flux/integrations/client/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/weaveworks/flux/integrations/client/listers/helm.integrations.flux.weave.works/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
	time "time"
)

// FluxHelmReleaseInformer provides access to a shared informer and lister for
// FluxHelmReleases.
type FluxHelmReleaseInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.FluxHelmReleaseLister
}

type fluxHelmReleaseInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewFluxHelmReleaseInformer constructs a new informer for FluxHelmRelease type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFluxHelmReleaseInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredFluxHelmReleaseInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredFluxHelmReleaseInformer constructs a new informer for FluxHelmRelease type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredFluxHelmReleaseInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.HelmV1alpha2().FluxHelmReleases(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.HelmV1alpha2().FluxHelmReleases(namespace).Watch(options)
			},
		},
		&helm_integrations_flux_weave_works_v1alpha2.FluxHelmRelease{},
		resyncPeriod,
		indexers,
	)
}

func (f *fluxHelmReleaseInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredFluxHelmReleaseInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *fluxHelmReleaseInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&helm_integrations_flux_weave_works_v1alpha2.FluxHelmRelease{}, f.defaultInformer)
}

func (f *fluxHelmReleaseInformer) Lister() v1alpha2.FluxHelmReleaseLister {
	return v1alpha2.NewFluxHelmReleaseLister(f.Informer().GetIndexer())
}
