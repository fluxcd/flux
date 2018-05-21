/*
When a Chart release is manually deleted/upgraded/created, the cluster gets out of sync
with the prescribed state defined by FluxHelmRelease custom resources. The releasesync package
attemps to bring the cluster back to the prescribed state.
*/
package releasesync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	protobuf "github.com/golang/protobuf/ptypes/timestamp"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/go-kit/kit/log"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	"github.com/weaveworks/flux/integrations/helm/customresource"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

const (
	syncDelay = 90
)

type releaseFhr struct {
	RelName string
	Fhr     ifv1.FluxHelmRelease
}

// ReleaseChangeSync implements DoReleaseChangeSync to return the cluster to the
// state dictated by Custom Resources after manual Chart release(s).
type ReleaseChangeSync struct {
	logger  log.Logger
	release chartrelease.Releaser
}

// New creates a ReleaseChangeSync.
func New(
	logger log.Logger,
	releaser chartrelease.Releaser) *ReleaseChangeSync {

	return &ReleaseChangeSync{
		logger:  logger,
		release: releaser,
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
// after manual Chart release(s).
func (rs *ReleaseChangeSync) DoReleaseChangeSync(ifClient ifclientset.Clientset, ns []string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
	relsToSync, err := rs.releasesToSync(ctx, ifClient, ns)
	cancel()
	if err != nil {
		err = errors.Wrap(err, "getting info about manual chart release changes")
		rs.logger.Log("error", err)
		return false, err
	}
	if len(relsToSync) == 0 {
		return false, nil
	}

	ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
	err = rs.sync(ctx, relsToSync)
	cancel()
	if err != nil {
		return false, errors.Wrap(err, "syncing cluster after manual chart release changes")
	}

	return true, nil
}

// getCustomResources retrieves FluxHelmRelease resources
// and returns them organised by namespace and chart release name.
// map[namespace] = []releaseFhr.
func (rs *ReleaseChangeSync) getCustomResources(
	ifClient ifclientset.Clientset,
	namespaces []string) (map[string][]releaseFhr, error) {

	relInfo := make(map[string][]releaseFhr)

	for _, ns := range namespaces {
		list, err := customresource.GetNSCustomResources(ifClient, ns)
		if err != nil {
			return nil, errors.Wrap(err,
				fmt.Sprintf("retrieving FluxHelmReleases in namespace %s", ns))
		}

		rf := []releaseFhr{}
		for _, fhr := range list.Items {
			relName := chartrelease.GetReleaseName(fhr)
			rf = append(rf, releaseFhr{RelName: relName, Fhr: fhr})
		}
		if len(rf) > 0 {
			relInfo[ns] = rf
		}
	}
	return relInfo, nil
}

// shouldUpgrade returns true if the current running values or chart
// don't match what the repo says we ought to be running, based on
// doing a dry run install from the chart in the git repo.
func (rs *ReleaseChangeSync) shouldUpgrade(
	currRel *hapi_release.Release,
	fhr ifv1.FluxHelmRelease) (bool, error) {

	if currRel == nil {
		return false, fmt.Errorf("No Chart release provided for %v", fhr.GetName())
	}

	currVals := currRel.GetConfig().GetRaw()
	currChart := currRel.GetChart().String()

	// Get the desired release state
	opts := chartrelease.InstallOptions{DryRun: true}
	tempRelName := strings.Join([]string{currRel.GetName(), "temp"}, "-")
	desRel, err := rs.release.Install(rs.release.ConfigSync(), tempRelName, fhr, "CREATE", opts)
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

// addExistingReleasesToSync populates relsToSync (map from namespace
// to chartRelease) with the members of currentReleases that need
// updating because they're diverged from the desired state.  Desired
// state is specified by customResources and what's in our git checkout.
func (rs *ReleaseChangeSync) addExistingReleasesToSync(
	relsToSync map[string][]chartRelease,
	currentReleases map[string]map[string]struct{},
	customResources map[string]map[string]ifv1.FluxHelmRelease) error {

	var chRels []chartRelease
	for ns, nsRelsM := range currentReleases {
		chRels = relsToSync[ns]
		for relName := range nsRelsM {
			if customResources[ns] == nil {
				continue
			}
			// We are ignoring Charts that are not under flux/helm-operator control
			fhr, ok := customResources[ns][relName]
			if !ok {
				continue
			}
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
		if len(chRels) > 0 {
			relsToSync[ns] = chRels
		}
	}
	return nil
}

// addDeletedReleasesToSync populates relsToSync (map from namespace
// to chartRelease) with chartReleases based on the charts referenced
// in customResources that are absent from currentReleases.
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

// releasesToSync queries Tiller to get all current Helm releases, queries k8s
// custom resources to get all FluxHelmRelease(s), and returns a map from
// namespace to chartRelease(s) that need to be synced.
func (rs *ReleaseChangeSync) releasesToSync(
	ctx context.Context,
	ifClient ifclientset.Clientset,
	ns []string) (map[string][]chartRelease, error) {

	relDepl, err := rs.release.GetCurrent()
	if err != nil {
		return nil, err
	}
	curRels := mappifyDeployInfo(relDepl)

	relCrs, err := rs.getCustomResources(ifClient, ns)
	if err != nil {
		return nil, err
	}
	crs := mappifyReleaseFhrInfo(relCrs)

	relsToSync := make(map[string][]chartRelease)

	// FIXME: we probably shouldn't be throwing away errors
	_ = rs.addDeletedReleasesToSync(relsToSync, curRels, crs)
	_ = rs.addExistingReleasesToSync(relsToSync, curRels, crs)

	return relsToSync, nil
}

// sync takes a map from namespace to list of chartRelease(s)
// that need to be applied, and attempts to apply them.
// It returns the first error encountered.  A chart missing
// from the repo doesn't count as an error, but will be logged.
func (rs *ReleaseChangeSync) sync(
	ctx context.Context,
	releases map[string][]chartRelease) error {

	// TODO it's weird that we do a pull here, after we've already decided
	// what to do.  Ask why.
	ctx, cancel := context.WithTimeout(ctx, helmgit.DefaultPullTimeout)
	err := rs.release.ConfigSync().Pull(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("Failure while pulling repo: %#v", err)
	}

	checkout := rs.release.ConfigSync()
	chartPathBase := filepath.Join(checkout.Dir, checkout.Config.Path)

	opts := chartrelease.InstallOptions{DryRun: false}

	for ns, relsToProcess := range releases {

		for _, chr := range relsToProcess {

			// sanity check
			chartPath := filepath.Join(chartPathBase, chr.desiredState.Spec.ChartGitPath)
			if _, err := os.Stat(chartPath); os.IsNotExist(err) {
				rs.logger.Log("error", fmt.Sprintf("Missing Chart %s. No release can happen.", chartPath))
				continue
			}

			relName := chr.releaseName
			switch chr.action {
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
			default:
				panic(fmt.Sprintf("invalid action %q", chr.action))
			}
		}
	}
	return nil
}
