package customresource

import (
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNSCustomResources(ifClient ifclientset.Clientset, ns string) (*ifv1.FluxHelmReleaseList, error) {
	return ifClient.HelmV1alpha2().FluxHelmReleases(ns).List(metav1.ListOptions{})
}
