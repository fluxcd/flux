package v1alpha

import (
	"bytes"
	"encoding/gob"

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
	ChartGitPath string         `json:"chartGitPath"`
	ReleaseName  string         `json:"releaseName,omitempty"`
	Values       FluxHelmValues `json:"values,omitempty"`
}

// FluxHelmValues embeds chartutil.Values so we can implement deepcopy on map[string]interface{}
// +k8s:deepcopy-gen=false
type FluxHelmValues struct {
	chartutil.Values
}

// DeepCopyInto implements deepcopy-gen method for use in generated code
func (v FluxHelmValues) DeepCopyInto(out *FluxHelmValues) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	dec := gob.NewDecoder(&buf)
	err := enc.Encode(v)
	if err != nil {
		return
	}
	err = dec.Decode(&out)
	if err != nil {
		return
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FluxHelmReleaseList is a list of FluxHelmRelease resources
type FluxHelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []FluxHelmRelease `json:"items"`
}
