package v1alpha

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmRelease represents custom resource associated with a Helm Chart
type FluxHelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec FluxHelmReleaseSpec `json:"spec"`
}

// FluxHelmReleaseSpec is the spec for a FluxHelmRelease resource
// FluxHelmReleaseSpec
type FluxHelmReleaseSpec struct {
	ChartGitPath string           `json:"chartGitPath"`
	ReleaseName  string           `json:"releaseName,omitempty"`
	Values       []HelmChartParam `json:"values,omitempty"`
}

// HelmChartParam represents Helm Chart customization
// 	it will be applied to override the values.yaml and/or the Chart itself
//		Name  ... parameter name; if missing this parameter will be discarded
//		Value ...

// HelmChartParam ... user customization of Chart parameterized values
type HelmChartParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmReleaseList is a list of FluxHelmRelease resources
type FluxHelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FluxHelmRelease `json:"items"`
}
