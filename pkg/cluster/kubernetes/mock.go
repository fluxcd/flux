package kubernetes

import kresource "github.com/fluxcd/flux/pkg/cluster/kubernetes/resource"

type ConstNamespacer string

func (ns ConstNamespacer) EffectiveNamespace(manifest kresource.KubeManifest, _ ResourceScopes) (string, error) {
	return string(ns), nil
}
