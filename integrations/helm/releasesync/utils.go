package releasesync

import (
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	"github.com/weaveworks/flux/integrations/helm/release"
)

func MappifyDeployInfo(releases map[string][]release.DeployInfo) map[string]map[string]struct{} {
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

func MappifyReleaseFhrInfo(fhrs map[string][]ReleaseFhr) map[string]map[string]ifv1.FluxHelmRelease {
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
