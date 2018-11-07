// +k8s:deepcopy-gen=package,register
// +groupName=flux.weave.works
// Package v1beta1 is the v1beta1 version of the API. The prior
// version was helm.integrations.flux.weave.works/v1alpha2.
//
// This version has breaking changes, but since it is in a different
// API group entirely, it can coexist with the old version. You may
// need to take care to use the full kind (including the group) to
// refer to resources, e.g., `fluxhelmrelease.flux.weave.works`.
package v1beta1
