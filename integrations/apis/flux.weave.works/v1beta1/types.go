package v1beta1

import (
	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/helm/pkg/chartutil"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmRelease represents custom resource associated with a Helm Chart
type FluxHelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   FluxHelmReleaseSpec   `json:"spec"`
	Status FluxHelmReleaseStatus `json:"status"`
}

type ChartSource struct {
	// one of the following...
	// +optional
	*GitChartSource
	// +optional
	*RepoChartSource
}

type GitChartSource struct {
	// URL would go here, and possibly SSH key secret; but, not yet.

	// Path to the chart. This has the name `gitPath` because there is
	// nothing enclosing to disambiguate what the path refers to.
	Path string `json:"gitPath"`
}

type RepoChartSource struct {
	RepoURL string `json:"repository"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// An authentication secret for accessing the chart repo
	// +optional
	ChartPullSecret *v1.LocalObjectReference `json:"chartPullSecret,omitempty"`
}

// FluxHelmReleaseSpec is the spec for a FluxHelmRelease resource
// FluxHelmReleaseSpec
type FluxHelmReleaseSpec struct {
	ChartSource `json:"chart"`
	ReleaseName string `json:"releaseName,omitempty"`

	ValueFileSecrets []v1.LocalObjectReference `json:"valueFileSecrets,omitempty"`
	FluxHelmValues   `json:",inline"`
}

type FluxHelmReleaseStatus struct {
	ReleaseStatus string `json:"releaseStatus"`
}

// FluxHelmValues embeds chartutil.Values so we can implement deepcopy on map[string]interface{}
// +k8s:deepcopy-gen=false
type FluxHelmValues struct {
	chartutil.Values `json:"values,omitempty"`
}

// DeepCopyInto implements deepcopy-gen method for use in generated code
func (in *FluxHelmValues) DeepCopyInto(out *FluxHelmValues) {
	if in == nil {
		return
	}

	b, err := yaml.Marshal(in.Values)
	if err != nil {
		return
	}
	var values chartutil.Values
	err = yaml.Unmarshal(b, &values)
	if err != nil {
		return
	}
	out.Values = values
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmReleaseList is a list of FluxHelmRelease resources
type FluxHelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FluxHelmRelease `json:"items"`
}
