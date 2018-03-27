/*
When a Chart release is deleted/upgraded/created manually, the cluster gets out of sync
with the prescribed state defined in the get repo.
*/
package releasesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/weaveworks/flux/integrations/helm/chartsync"
	"github.com/weaveworks/flux/integrations/helm/release"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	"github.com/go-kit/kit/log"

	//"gopkg.in/src-d/go-git.v4/plumbing"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned" // kubernetes 1.9
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type action string

const (
	CustomResourceKind        = "FluxHelmRelease"
	releaseLagTime            = 30
	deleteAction       action = "DELETE"
	installAction      action = "CREATE"
	upgradeAction      action = "UPDATE"
)

type ReleaseFhr struct {
	RelName string
	FhrName string
	Fhr     ifv1.FluxHelmRelease
}

// ReleaseChangeSync will become a receiver, that contains
//
type ReleaseChangeSync struct {
	logger log.Logger
	chartsync.Polling
	kubeClient kubernetes.Clientset
	//chartSync  chartsync.ChartChangeSync
	ifClient ifclientset.Clientset
	//fhrLister iflister.FluxHelmReleaseLister
	release *chartrelease.Release
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
	action       action
	desiredState ifv1.FluxHelmRelease
}

// Run ... creates a syncing loop monitoring repo chart changes
func (rs *ReleaseChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	rs.logger.Log("info", "Starting repo charts sync loop")

	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer wg.Done()
		defer rs.release.Repo.ReleasesSync.Cleanup()

		time.Sleep(30 * time.Second)

		ticker := time.NewTicker(rs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			// ------------------------------------------------------------------------------------
			case <-ticker.C:
				fmt.Printf("\n\t... RELEASESYNC at %s\n\n", time.Now().String())
				ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				relsToSync, err := rs.releasesToSync(ctx)
				cancel()
				if err != nil {
					rs.logger.Log("error", fmt.Sprintf("Failure to get info about manual chart release changes: %#v", err))
					fmt.Printf("\n\t... RELEASESYNC work FINISHED at %s\n\n", time.Now().String())
					continue
				}

				// manual chart release changes?
				if len(relsToSync) == 0 {
					rs.logger.Log("info", fmt.Sprintln("No manual changes of Chart releases"))
					fmt.Printf("\n\t... RELEASESYNC work FINISHED at %s\n\n", time.Now().String())
					continue
				}

				// sync Chart releases
				ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				err = rs.sync(ctx, relsToSync)
				cancel()
				if err != nil {
					rs.logger.Log("error", fmt.Sprintf("Failure to sync cluster after manual chart release changes: %#v", err))
				}
				fmt.Printf("\n\t... RELEASESYNC work FINISHED at %s\n\n", time.Now().String())
			// ------------------------------------------------------------------------------------
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

	fmt.Println("\nGETTING CUSTOM RESOURCES")
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
			//fmt.Printf(">>> existing FHRs    ns: %s ... relName: %s\n", ns, relName)
		}
		if len(rf) > 0 {
			relInfo[ns] = rf
		}
	}
	return relInfo, nil
}

// getReleaseEvents retrieves and stores last event timestamp for FluxHelmRelease resources (FHR)
//		output:
//						map[namespace][FHR name] = int64
func (rs *ReleaseChangeSync) getEventsLastTimestamp(namespaces []string) (map[string]map[string]int64, error) {
	relEventsTime := make(map[string]map[string]int64)

	fmt.Println("\nGETTING CUSTOM RESOURCE EVENTS")
	for _, ns := range namespaces {
		eventList, err := rs.getNSEvents(ns)
		if err != nil {
			return relEventsTime, err
		}
		fhrD := make(map[string]int64)
		for _, e := range eventList.Items {
			if e.InvolvedObject.Kind == CustomResourceKind {
				secs := e.LastTimestamp.Unix()
				fhrD[e.InvolvedObject.Name] = secs

				//fmt.Printf("<<< existing FHR Events    ns: %s ... fhrName: %s ... %d (%s)\n", ns, e.InvolvedObject.Name, secs, e.LastTimestamp.String())
				//fmt.Printf("\t<<< map of existing FHR Events    ns: %s \n\n\t%+v\n\n", ns, fhrD)
			}
		}
		relEventsTime[ns] = fhrD
	}
	return relEventsTime, nil
}

// existingReleasesToSync determines which Chart releases need to be deleted/upgraded
// to bring the cluster to the desired state
func (rs *ReleaseChangeSync) existingReleasesToSync(
	currentReleases map[string]map[string]int64,
	customResources map[string]map[string]ifv1.FluxHelmRelease,
	events map[string]map[string]int64,
	relsToSync map[string][]chartRelease) error {

	var chRels []chartRelease
	for ns, nsRelsM := range currentReleases {
		chRels = relsToSync[ns]
		for relName, relUpdated := range nsRelsM {
			fhr, ok := customResources[ns][relName]
			if !ok {
				chr := chartRelease{
					releaseName:  relName,
					action:       deleteAction,
					desiredState: fhr,
				}
				chRels = append(chRels, chr)
			} else {
				fhrUpdated := events[ns][fhr.Name]
				rs.logger.Log("info", fmt.Sprintf("release last deploy: %d ... FHR last deploy: %d => relUpdated - fhrUpdated: %d secs\n\n", relUpdated, fhrUpdated, relUpdated-fhrUpdated))

				if (relUpdated - fhrUpdated) > releaseLagTime {
					chr := chartRelease{
						releaseName:  relName,
						action:       upgradeAction,
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
func (rs *ReleaseChangeSync) deletedReleasesToSync(
	customResources map[string]map[string]ifv1.FluxHelmRelease,
	currentReleases map[string]map[string]int64,
	relsToSync map[string][]chartRelease) error {

	var chRels []chartRelease
	for ns, nsFhrs := range customResources {
		chRels = relsToSync[ns]

		for relName, fhr := range nsFhrs {
			// there are Custom Resources (CRs) in this namespace
			// missing Chart release even though there is a CR
			if _, ok := currentReleases[ns][relName]; !ok {
				chr := chartRelease{
					releaseName:  relName,
					action:       installAction,
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

// releasesToSync gathers all releases that need syncing
func (rs *ReleaseChangeSync) releasesToSync(ctx context.Context) (map[string][]chartRelease, error) {
	ns, err := chartsync.GetNamespaces(rs.logger, rs.kubeClient)
	if err != nil {
		return nil, err
	}
	relDepl, err := rs.release.GetCurrentWithDate()
	if err != nil {
		return nil, err
	}
	curRels := MappifyDeployInfo(relDepl)
	//rs.logger.Log("info", fmt.Sprintf("+++ curRels:\n\n%#v\n\n", curRels))
	//fmt.Sprintf("*** curRels:\n\n%+v\n\n", curRels)

	relCrs, err := rs.getCustomResources(ns)
	if err != nil {
		return nil, err
	}
	crs := MappifyReleaseFhrInfo(relCrs)
	//rs.logger.Log("info", fmt.Sprintf("+++ crs:\n\n%#v\n\n", crs))
	//fmt.Sprintf("*** crs:\n\n%+v\n\n", crs)

	evs, err := rs.getEventsLastTimestamp(ns)
	if err != nil {
		return nil, err
	}

	relsToSync := make(map[string][]chartRelease)
	rs.deletedReleasesToSync(crs, curRels, relsToSync)
	rs.existingReleasesToSync(curRels, crs, evs, relsToSync)

	return relsToSync, nil
}

// sync deletes/upgrades a Chart release
func (rs *ReleaseChangeSync) sync(ctx context.Context, releases map[string][]chartRelease) error {

	checkout := rs.release.Repo.ReleasesSync
	for ns, relsToProcess := range releases {
		for _, chr := range relsToProcess {
			relName := chr.releaseName
			switch chr.action {
			case deleteAction:
				rs.logger.Log("info", fmt.Sprintf("Deleting manually installed Chart release %s (namespace %s)", relName, ns))
				err := rs.release.Delete(relName)
				if err != nil {
					return err
				}
			case upgradeAction:
				rs.logger.Log("info", fmt.Sprintf("Resyncing manually upgraded Chart release %s (namespace %s)", relName, ns))
				_, err := rs.release.Install(checkout, relName, chr.desiredState, chartrelease.ReleaseType("UPDATE"), false)
				if err != nil {
					return err
				}
			case installAction:
				rs.logger.Log("info", fmt.Sprintf("Installing manually deleted Chart release %s (namespace %s)", relName, ns))
				_, err := rs.release.Install(checkout, relName, chr.desiredState, chartrelease.ReleaseType("CREATE"), false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
