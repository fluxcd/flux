package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmResource represents custom resource associated with a Helm Chart
type FluxHelmResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec FluxHelmResourceSpec `json:"spec"`
	//	Status FluxHelmResourceStatus `json:"status"`
}

// FluxHelmResourceSpec is the spec for a FluxHelmResource resource
// FluxHelmResourceSpec
type FluxHelmResourceSpec struct {
	ChartGitPath   string           `json:"chartgitpath"`
	ReleaseName    string           `json:"releasename,omitempty"`
	Customizations []HelmChartParam `json:"customizations,omitempty"`
}

// HelmChartParam represents Helm Chart customization
// 	it will be applied to override the values.yaml and/or the Chart itself
//		Name  ... parameter name; if missing this parameter will be discarded
//		Value ...
//		Type  ... type: string, integer, float; if missing, then string is the default

// HelmChartParam ... user customization of Chart parameterized values
type HelmChartParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

/*
// FluxHelmResourceStatus is the status for a FluxHelmResource resource
type FluxHelmResourceStatus struct {
	Revision string `json:"revision"`
}
*/

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmResourceList is a list of FluxHelmResource resources
type FluxHelmResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FluxHelmResource `json:"items"`
}
