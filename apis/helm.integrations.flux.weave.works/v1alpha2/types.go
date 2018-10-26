package v1alpha2

import (
	"github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
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

// FluxHelmReleaseSpec is the spec for a FluxHelmRelease resource
// FluxHelmReleaseSpec
type FluxHelmReleaseSpec struct {
	ChartGitPath     string                    `json:"chartGitPath"`
	ReleaseName      string                    `json:"releaseName,omitempty"`
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
