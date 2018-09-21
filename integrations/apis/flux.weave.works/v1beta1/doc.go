// +k8s:deepcopy-gen=package,register
// +groupName=flux.weave.works
// Package v1beta1 is the v1beta1 version of the API. The prior
// version was v1alpha2. This version has breaking changes, and since
// Kubernetes prior to v1.11 doesn't support multiple versions of
// custom resources, one must run either the old version or the new
// version (with their respective operators), and not mix them.
package v1beta1
