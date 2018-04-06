/*
When a Chart release is manually deleted/upgraded/created, the cluster gets out of sync
with the prescribed state defined by FluxHelmRelease custom resources. The releasesync package
attemps to bring the cluster back to the prescribed state.
*/
package releasesync

import (
	"context"
	"fmt"
	"strings"

	protobuf "github.com/golang/protobuf/ptypes/timestamp"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/go-kit/kit/log"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	"github.com/weaveworks/flux/integrations/helm/customresource"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

const (
	CustomResourceKind = "FluxHelmRelease"
	syncDelay          = 90
)

type ReleaseFhr struct {
	RelName string
	FhrName string
	Fhr     ifv1.FluxHelmRelease
}

type ReleaseChangeSync struct {
	logger  log.Logger
	release *chartrelease.Release
}

func New(
	logger log.Logger,
	release *chartrelease.Release) *ReleaseChangeSync {

	return &ReleaseChangeSync{
		logger:  logger,
		release: release,
	}
}

type customResourceInfo struct {
	name, releaseName string
	resource          ifv1.FluxHelmRelease
	lastUpdated       protobuf.Timestamp
}

type chartRelease struct {
	releaseName  string
	action       chartrelease.Action
	desiredState ifv1.FluxHelmRelease
}

// DoReleaseChangeSync returns the cluster to the state dictated by Custom Resources
// after manual Chart release(s)
func (rs *ReleaseChangeSync) DoReleaseChangeSync(ifClient ifclientset.Clientset, ns []string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
	relsToSync, err := rs.releasesToSync(ctx, ifClient, ns)
	cancel()
	if err != nil {
		err := fmt.Errorf("Failure to get info about manual chart release changes: %#v", err)
		return false, err
	}
	if len(relsToSync) == 0 {
		return false, nil
	}
	// sync Chart releases
	ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultPullTimeout)
	err = rs.release.Repo.ChartSync.Pull(ctx)
	cancel()
	if err != nil {
		return false, fmt.Errorf("Failure while pulling repo: %#v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
	err = rs.sync(ctx, relsToSync)
	cancel()
	if err != nil {
		err := fmt.Errorf("Failure to sync cluster after manual chart release changes: %#v", err)
		return false, err
	}

	return true, nil
}

// getCustomResources retrieves FluxHelmRelease resources
//		and outputs them organised by namespace and: Chart release name or Custom Resource name
//						map[namespace] = []ReleaseFhr
func (rs *ReleaseChangeSync) getCustomResources(ifClient ifclientset.Clientset, namespaces []string) (map[string][]ReleaseFhr, error) {
	relInfo := make(map[string][]ReleaseFhr)

	for _, ns := range namespaces {
		list, err := customresource.GetNSCustomResources(ifClient, ns)
		if err != nil {
			rs.logger.Log("error", fmt.Errorf("Failure while retrieving FluxHelmReleases in namespace %s: %v", ns, err))
			return nil, err
		}
		rf := []ReleaseFhr{}
		for _, fhr := range list.Items {
			relName := chartrelease.GetReleaseName(fhr)
			rf = append(rf, ReleaseFhr{RelName: relName, Fhr: fhr})
		}
		if len(rf) > 0 {
			relInfo[ns] = rf
		}
	}
	return relInfo, nil
}

func (rs *ReleaseChangeSync) shouldUpgrade(currRel *hapi_release.Release, fhr ifv1.FluxHelmRelease) (bool, error) {
	if currRel == nil {
		return false, fmt.Errorf("No Chart release provided for %v", fhr.GetName())
	}

	currVals := currRel.GetConfig().GetRaw()
	currChart := currRel.GetChart().String()

	// Get the desired release state
	opts := chartrelease.InstallOptions{DryRun: true}
	tempRelName := strings.Join([]string{currRel.GetName(), "temp"}, "-")
	desRel, err := rs.release.Install(rs.release.Repo.ChartSync, tempRelName, fhr, "CREATE", opts)
	if err != nil {
		return false, err
	}
	desVals := desRel.GetConfig().GetRaw()
	desChart := desRel.GetChart().String()

	// compare values && Chart
	if currVals != desVals {
		rs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()))
		return true, nil
	}
	if currChart != desChart {
		rs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()))
		return true, nil
	}

	return false, nil
}

// existingReleasesToSync determines which Chart releases need to be deleted/upgraded
// to bring the cluster to the desired state
func (rs *ReleaseChangeSync) addExistingReleasesToSync(
	relsToSync map[string][]chartRelease,
	currentReleases map[string]map[string]struct{},
	customResources map[string]map[string]ifv1.FluxHelmRelease) error {

	var chRels []chartRelease
	for ns, nsRelsM := range currentReleases {
		chRels = relsToSync[ns]
		for relName := range nsRelsM {
			if customResources[ns] == nil {
				chr := chartRelease{
					releaseName:  relName,
					action:       chartrelease.DeleteAction,
					desiredState: ifv1.FluxHelmRelease{},
				}
				chRels = append(chRels, chr)
				continue
			}
			fhr, ok := customResources[ns][relName]
			if !ok {
				chr := chartRelease{
					releaseName:  relName,
					action:       chartrelease.DeleteAction,
					desiredState: fhr,
				}
				chRels = append(chRels, chr)

			} else {
				rel, err := rs.release.GetDeployedRelease(relName)
				if err != nil {
					return err
				}
				doUpgrade, err := rs.shouldUpgrade(rel, fhr)
				if err != nil {
					return err
				}
				if doUpgrade {
					chr := chartRelease{
						releaseName:  relName,
						action:       chartrelease.UpgradeAction,
						desiredState: fhr,
					}
					chRels = append(chRels, chr)
				}
			}
		}
		if len(chRels) > 0 {
			relsToSync[ns] = chRels
		}
	}
	return nil
}

// deletedReleasesToSync determines which Chart releases need to be installed
// to bring the cluster to the desired state
func (rs *ReleaseChangeSync) addDeletedReleasesToSync(
	relsToSync map[string][]chartRelease,
	currentReleases map[string]map[string]struct{},
	customResources map[string]map[string]ifv1.FluxHelmRelease) error {

	var chRels []chartRelease
	for ns, nsFhrs := range customResources {
		chRels = relsToSync[ns]

		for relName, fhr := range nsFhrs {
			// there are Custom Resources (CRs) in this namespace
			// missing Chart release even though there is a CR
			if currentReleases[ns] == nil {
				chr := chartRelease{
					releaseName:  relName,
					action:       chartrelease.InstallAction,
					desiredState: fhr,
				}
				chRels = append(chRels, chr)
				continue
			}
			if _, ok := currentReleases[ns][relName]; !ok {
				chr := chartRelease{
					releaseName:  relName,
					action:       chartrelease.InstallAction,
					desiredState: fhr,
				}
				chRels = append(chRels, chr)
			}
		}
		if len(chRels) > 0 {
			relsToSync[ns] = chRels
		}
	}
	return nil
}

func (rs *ReleaseChangeSync) releasesToSync(ctx context.Context, ifClient ifclientset.Clientset, ns []string) (map[string][]chartRelease, error) {
	relDepl, err := rs.release.GetCurrent()
	if err != nil {
		return nil, err
	}
	curRels := MappifyDeployInfo(relDepl)

	relCrs, err := rs.getCustomResources(ifClient, ns)
	if err != nil {
		return nil, err
	}
	crs := MappifyReleaseFhrInfo(relCrs)

	relsToSync := make(map[string][]chartRelease)
	rs.addDeletedReleasesToSync(relsToSync, curRels, crs)
	rs.addExistingReleasesToSync(relsToSync, curRels, crs)

	return relsToSync, nil
}

func (rs *ReleaseChangeSync) sync(ctx context.Context, releases map[string][]chartRelease) error {

	checkout := rs.release.Repo.ChartSync
	opts := chartrelease.InstallOptions{DryRun: false}
	for ns, relsToProcess := range releases {
		for _, chr := range relsToProcess {
			relName := chr.releaseName
			switch chr.action {
			case chartrelease.DeleteAction:
				rs.logger.Log("info", fmt.Sprintf("Deleting manually installed Chart release %s (namespace %s)", relName, ns))
				err := rs.release.Delete(relName)
				if err != nil {
					return err
				}
			case chartrelease.UpgradeAction:
				rs.logger.Log("info", fmt.Sprintf("Resyncing manually upgraded Chart release %s (namespace %s)", relName, ns))
				_, err := rs.release.Install(checkout, relName, chr.desiredState, chartrelease.UpgradeAction, opts)
				if err != nil {
					return err
				}
			case chartrelease.InstallAction:
				rs.logger.Log("info", fmt.Sprintf("Installing manually deleted Chart release %s (namespace %s)", relName, ns))
				_, err := rs.release.Install(checkout, relName, chr.desiredState, chartrelease.InstallAction, opts)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
