/*
When a Chart release is deleted/upgraded/created manually, the cluster gets out of sync
with the prescribed state defined in the get repo.
*/
package releasesync

import (
	"fmt"
	"time"

	protobuf "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/weaveworks/flux/integrations/helm/chartsync"
	"github.com/weaveworks/flux/integrations/helm/release"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/go-kit/kit/log"

	//"gopkg.in/src-d/go-git.v4/plumbing"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	iflister "github.com/weaveworks/flux/integrations/client/listers/helm.integrations.flux.weave.works/v1alpha" // kubernetes 1.9
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type action string

const (
	CustomResourceKind        = "FluxHelmRelease"
	releaseLagTime            = int64(60 * time.Second)
	deleteAction       action = "DELETE"
	installAction      action = "INSTALL"
	upgradeAction      action = "UPGRADE"
)

// Polling allows to specify polling criteria
type Polling struct {
	Interval time.Duration
	Timeout  time.Duration
}

// ReleaseChangeSync will become a receiver, that contains
//
type ReleaseChangeSync struct {
	logger log.Logger
	Polling
	kubeClient kubernetes.Clientset
	chartSync  chartsync.ChartChangeSync
	ifClient   ifclientset.Clientset
	fhrLister  iflister.FluxHelmReleaseLister
	release    *chartrelease.Release
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

func (rs *ReleaseChangeSync) getNSCustomResources(ns string) (*ifv1.FluxHelmReleaseList, error) {
	return rs.ifClient.HelmV1alpha().FluxHelmReleases(ns).List(metav1.ListOptions{})
}

func (rs *ReleaseChangeSync) getNSEvents(ns string) (*v1.EventList, error) {
	return rs.kubeClient.CoreV1().Events(ns).List(metav1.ListOptions{})
}

// getCustomResources retrieves FluxHelmRelease resources
//		and outputs them organised by namespace and: Chart release name or Custom Resource name
//						map[namespace]["fhrName"][FHR name]     = ifv1.FluxHelmRelease
//						map[namespace]["relName"][release name] = ifv1.FluxHelmRelease
func (rs *ReleaseChangeSync) getCustomResources(namespaces []string) map[string]map[string]map[string]ifv1.FluxHelmRelease {
	var relInfo map[string]map[string]map[string]ifv1.FluxHelmRelease

	for _, ns := range namespaces {
		list, err := rs.getNSCustomResources(ns)
		if err != nil {
			rs.logger.Log("error", fmt.Errorf("Failure while retrieving FluxHelmReleases in namespace %s: %v", ns, err))
			continue
		}
		var relM map[string]map[string]ifv1.FluxHelmRelease
		var inRelNameM map[string]ifv1.FluxHelmRelease
		var inFhrNameM map[string]ifv1.FluxHelmRelease
		for _, fhr := range list.Items {
			relName := release.GetReleaseName(fhr)

			inRelNameM[relName] = fhr
			inFhrNameM[fhr.Name] = fhr

			relM["relName"] = inRelNameM
			relM["fhrName"] = inFhrNameM
			relInfo[ns] = relM
		}
	}
	return relInfo
}

// getReleaseEvents retrieves and stores last event timestamp for FluxHelmRelease resources (FHR)
//		output:
//						map[namespace][FHR name] = time.Unix() [int64]
func (rs *ReleaseChangeSync) getEventsLastTimestamp(namespaces []string) (map[string]map[string]int64, error) {
	var relEventsTime map[string]map[string]int64

	for _, ns := range namespaces {
		eventList, err := rs.getNSEvents(ns)
		if err != nil {
			return relEventsTime, err
		}
		for _, e := range eventList.Items {
			if e.InvolvedObject.Kind == CustomResourceKind {
				secs := e.LastTimestamp.Unix()
				relEventsTime[ns] = map[string]int64{e.InvolvedObject.Name: secs}
			}
		}
	}
	return relEventsTime, nil
}

// existingReleasesToSync determines which Chart releases need to be deleted/upgraded
// to bring the cluster to the desired state
// TODO: more thinking
func (rs *ReleaseChangeSync) existingReleasesToSync(
	currentReleases map[string]map[string]int64,
	customResources map[string]map[string]map[string]ifv1.FluxHelmRelease,
	events map[string]map[string]int64) (map[string][]chartRelease, error) {
	/*namespaces, err := chartsync.GetNamespaces(rs.logger, kubeClient)
	if err != nil {
		return map[string][]chartRelease{}, err
	}
	*/
	var relsToSync map[string][]chartRelease
	var chRels []chartRelease

	for ns, nsRelsM := range currentReleases {
		for relName, relUpdated := range nsRelsM {
			fhr, ok := customResources[ns]["relName"][relName]
			if !ok {
				chr := chartRelease{
					releaseName:  relName,
					action:       deleteAction,
					desiredState: fhr,
				}
				chRels = append(chRels, chr)
			} else {
				// TODO: more thinking
				fhrUpdated := events[ns][fhr.Name]
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
		relsToSync[ns] = chRels
	}
	return relsToSync, nil
}

// deletedReleasesToSync determines which Chart releases need to be installed
// to bring the cluster to the desired state
//
func (rs *ReleaseChangeSync) deletedReleasesToSync(
	namespaces []string,
	customResources map[string]map[string]map[string]ifv1.FluxHelmRelease,
	currentReleases map[string]map[string]int64) (map[string][]chartRelease, error) {

	var relsToSync map[string][]chartRelease
	var chRels []chartRelease
	for _, ns := range namespaces {
		// there are Custom Resources (CRs) in this namespace
		if nsRels, ok := customResources[ns]["relName"]; ok {
			for relName := range nsRels {
				// missing Chart release even though there is a CR
				if _, ok := currentReleases[ns][relName]; !ok {
					chr := chartRelease{
						releaseName:  relName,
						action:       installAction,
						desiredState: nsRels[relName],
					}
					chRels = append(chRels, chr)
				}
			}
		}
		relsToSync[ns] = chRels
	}

	return relsToSync, nil
}

// sync deletes/upgrades a Chart release
func (rs *ReleaseChangeSync) sync(releases map[string][]chartRelease) error {
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
				_, err := rs.release.Install(checkout, relName, chr.desiredState, chartrelease.ReleaseType("INSTALL"), false)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
