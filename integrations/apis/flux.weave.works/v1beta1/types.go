package v1beta1

import (
	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/helm/pkg/chartutil"

	"github.com/weaveworks/flux"
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

// ResourceID returns an ID made from the identifying parts of the
// resource, as a convenience for Flux, which uses them
// everywhere.
func (fhr FluxHelmRelease) ResourceID() flux.ResourceID {
	return flux.MakeResourceID(fhr.Namespace, "FluxHelmRelease", fhr.Name)
}

type ChartSource struct {
	// one of the following...
	// +optional
	*GitChartSource
	// +optional
	*RepoChartSource
}

type GitChartSource struct {
	GitURL string `json:"git"`
	Ref    string `json:"ref"`
	Path   string `json:"path"`
}

// DefaultGitRef is the ref assumed if the Ref field is not given in a GitChartSource
const DefaultGitRef = "master"

func (s GitChartSource) RefOrDefault() string {
	if s.Ref == "" {
		return DefaultGitRef
	}
	return s.Ref
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
	// ReleaseName is the name as either supplied or generated.
	// +optional
	ReleaseName string `json:"releaseName"`

	// ReleaseStatus is the status as given by Helm for the release
	// managed by this resource.
	ReleaseStatus string `json:"releaseStatus"`

	// Conditions contains observations of the resource's state, e.g.,
	// has the chart which it refers to been fetched.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []FluxHelmReleaseCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type FluxHelmReleaseCondition struct {
	Type   FluxHelmReleaseConditionType `json:"type"`
	Status v1.ConditionStatus           `json:"status"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

type FluxHelmReleaseConditionType string

const (
	// ChartFetched means the chart to which the FluxHelmRelease
	// refers has been fetched successfully
	FluxHelmReleaseChartFetched FluxHelmReleaseConditionType = "ChartFetched"
	// Released means the chart release, as specified in this
	// FluxHelmRelease, has been processed by Helm.
	FluxHelmReleaseReleased FluxHelmReleaseConditionType = "Released"
)

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
