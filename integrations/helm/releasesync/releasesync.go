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
	"sync"
	"time"

	protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/weaveworks/flux/integrations/helm/chartsync"
	"github.com/weaveworks/flux/integrations/helm/release"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/go-kit/kit/log"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

const (
	CustomResourceKind = "FluxHelmRelease"
	syncDelay          = 60
)

type ReleaseFhr struct {
	RelName string
	FhrName string
	Fhr     ifv1.FluxHelmRelease
}

type ReleaseChangeSync struct {
	logger log.Logger
	chartsync.Polling
	kubeClient kubernetes.Clientset
	ifClient   ifclientset.Clientset
	release    *chartrelease.Release
}

func New(
	logger log.Logger, syncInterval time.Duration, syncTimeout time.Duration,
	kubeClient kubernetes.Clientset, ifClient ifclientset.Clientset,
	release *chartrelease.Release) *ReleaseChangeSync {

	return &ReleaseChangeSync{
		logger:     logger,
		Polling:    chartsync.Polling{Interval: syncInterval, Timeout: syncTimeout},
		kubeClient: kubeClient,
		ifClient:   ifClient,
		release:    release,
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

// Run creates a syncing loop monitoring repo chart changes
func (rs *ReleaseChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	rs.logger.Log("info", "Starting repo charts sync loop")

	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer wg.Done()
		defer rs.release.Repo.ReleasesSync.Cleanup()

		time.Sleep(syncDelay * time.Second)

		ticker := time.NewTicker(rs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rs.logger.Log("info", fmt.Sprint("Start of releasesync"))
				ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				relsToSync, err := rs.releasesToSync(ctx)
				cancel()
				if err != nil {
					rs.logger.Log("error", fmt.Sprintf("Failure to get info about manual chart release changes: %#v", err))
					rs.logger.Log("info", fmt.Sprint("End of releasesync"))
					continue
				}

				if len(relsToSync) == 0 {
					rs.logger.Log("info", fmt.Sprint("No manual changes of Chart releases"))
					rs.logger.Log("info", fmt.Sprint("End of releasesync"))
					continue
				}

				// sync Chart releases
				ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				err = rs.sync(ctx, relsToSync)
				cancel()
				if err != nil {
					rs.logger.Log("error", fmt.Sprintf("Failure to sync cluster after manual chart release changes: %#v", err))
				}
				rs.logger.Log("info", fmt.Sprint("End of releasesync"))
			case <-stopCh:
				rs.logger.Log("stopping", "true")
				break
			}
		}
	}()
}

func (rs *ReleaseChangeSync) getNSCustomResources(ns string) (*ifv1.FluxHelmReleaseList, error) {
	return rs.ifClient.HelmV1alpha().FluxHelmReleases(ns).List(metav1.ListOptions{})
}

func (rs *ReleaseChangeSync) getNSEvents(ns string) (*v1.EventList, error) {
	return rs.kubeClient.CoreV1().Events(ns).List(metav1.ListOptions{})
}

// getCustomResources retrieves FluxHelmRelease resources
//		and outputs them organised by namespace and: Chart release name or Custom Resource name
//						map[namespace] = []ReleaseFhr
func (rs *ReleaseChangeSync) getCustomResources(namespaces []string) (map[string][]ReleaseFhr, error) {
	relInfo := make(map[string][]ReleaseFhr)

	for _, ns := range namespaces {
		list, err := rs.getNSCustomResources(ns)
		if err != nil {
			rs.logger.Log("error", fmt.Errorf("Failure while retrieving FluxHelmReleases in namespace %s: %v", ns, err))
			return nil, err
		}
		rf := []ReleaseFhr{}
		for _, fhr := range list.Items {
			relName := release.GetReleaseName(fhr)
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
	desRel, err := rs.release.Install(rs.release.Repo.ReleasesSync, tempRelName, fhr, "CREATE", opts)
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

func (rs *ReleaseChangeSync) releasesToSync(ctx context.Context) (map[string][]chartRelease, error) {
	ns, err := chartsync.GetNamespaces(rs.logger, rs.kubeClient)
	if err != nil {
		return nil, err
	}
	relDepl, err := rs.release.GetCurrent()
	if err != nil {
		return nil, err
	}
	curRels := MappifyDeployInfo(relDepl)

	relCrs, err := rs.getCustomResources(ns)
	if err != nil {
		return nil, err
	}
	crs := MappifyReleaseFhrInfo(relCrs)

	relsToSync := make(map[string][]chartRelease)
	rs.addDeletedReleasesToSync(relsToSync, curRels, crs)
	rs.addExistingReleasesToSync(relsToSync, curRels, crs)

	return relsToSync, nil
}

// sync deletes/upgrades/installs a Chart release
func (rs *ReleaseChangeSync) sync(ctx context.Context, releases map[string][]chartRelease) error {

	checkout := rs.release.Repo.ReleasesSync
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
