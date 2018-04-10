package v1alpha

import (
	"github.com/ghodss/yaml"
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

	Spec FluxHelmReleaseSpec `json:"spec"`
}

// FluxHelmReleaseSpec is the spec for a FluxHelmRelease resource
// FluxHelmReleaseSpec
type FluxHelmReleaseSpec struct {
	ChartGitPath   string `json:"chartGitPath"`
	ReleaseName    string `json:"releaseName,omitempty"`
	FluxHelmValues `json:",inline"`
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
