package releasesync

import (
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	"github.com/weaveworks/flux/integrations/helm/release"
)

// mappifyDeployInfo takes a map of namespace -> []DeployInfo,
// returning a map whose keys are the same namespaces
// and whose values are key-only maps holding the DeployInfo names.
func mappifyDeployInfo(releases map[string][]release.DeployInfo) map[string]map[string]struct{} {
	deployM := make(map[string]map[string]struct{})

	for ns, nsRels := range releases {
		nsDeployM := make(map[string]struct{})
		for _, r := range nsRels {
			nsDeployM[r.Name] = struct{}{}
		}
		deployM[ns] = nsDeployM
	}
	return deployM
}

// mappifyReleaseFhrInfo takes a map of namespace -> []releaseFhr,
// returning a map whose keys are the same namespaces
// and whose values are maps of releaseName -> FluxHelmRelease.
func mappifyReleaseFhrInfo(fhrs map[string][]releaseFhr) map[string]map[string]ifv1.FluxHelmRelease {
	relFhrM := make(map[string]map[string]ifv1.FluxHelmRelease)

	for ns, nsFhrs := range fhrs {
		nsRels := make(map[string]ifv1.FluxHelmRelease)
		for _, r := range nsFhrs {
			nsRels[r.RelName] = r.Fhr
		}
		relFhrM[ns] = nsRels
	}

	return relFhrM
}
