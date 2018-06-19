/*
Package chartsync provides the functionality for updating a Chart release
due to (git repo) changes of Charts, while no Custom Resource changes.

Helm operator regularly checks the Chart repo and if new commits are found
all Custom Resources related to the changed Charts are updates, resulting in new
Chart release(s).
*/
package chartsync

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/weaveworks/flux/integrations/helm/releasesync"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type Polling struct {
	Interval time.Duration
	Timeout  time.Duration
}

type Clients struct {
	KubeClient kubernetes.Clientset
	IfClient   ifclientset.Clientset
}

type ChartChangeSync struct {
	logger log.Logger
	Polling
	kubeClient          kubernetes.Clientset
	ifClient            ifclientset.Clientset
	release             *chartrelease.Release
	relsync             releasesync.ReleaseChangeSync
	lastCheckedRevision string
}

func New(
	logger log.Logger,
	polling Polling,
	clients Clients,
	release *chartrelease.Release,
	relsync releasesync.ReleaseChangeSync) *ChartChangeSync {

	lastCheckedRevision := ""
	gitRef, err := release.Repo.ConfigSync.GetRevision()
	if err != nil {
		// we shall try again later
	}
	lastCheckedRevision = gitRef.String()

	return &ChartChangeSync{
		logger:              logger,
		Polling:             polling,
		kubeClient:          clients.KubeClient,
		ifClient:            clients.IfClient,
		release:             release,
		relsync:             relsync,
		lastCheckedRevision: lastCheckedRevision,
	}
}

//  Run creates a syncing loop monitoring repo chart changes
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	chs.logger.Log("info", "Starting charts sync loop")

	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer wg.Done()

		ticker := time.NewTicker(chs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ns, err := GetNamespaces(chs.logger, chs.kubeClient)
				if err != nil {
					errc <- err
				}

				var syncNeeded bool

				// Syncing git repo Charts only changes
				chs.logger.Log("info", fmt.Sprint("Start of chartsync"))
				syncNeeded, err = chs.DoChartChangeSync(ns)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do chart sync: %#v", err))
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
				}
				if !syncNeeded {
					chs.logger.Log("info", fmt.Sprint("No repo changes of Charts"))
				}
				chs.logger.Log("info", fmt.Sprint("End of chartsync"))

				// Syncing manual Chart releases
				chs.logger.Log("info", fmt.Sprint("Start of releasesync"))
				syncNeeded, err = chs.relsync.DoReleaseChangeSync(chs.ifClient, ns)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do manual release sync: %#v", err))
					chs.logger.Log("info", fmt.Sprint("End of releasesync"))
				}
				if !syncNeeded {
					chs.logger.Log("info", fmt.Sprint("No manual changes of Chart releases"))
				}
				chs.logger.Log("info", fmt.Sprint("End of releasesync"))
				continue

			case <-stopCh:
				chs.logger.Log("stopping", "true")
				break
			}
		}
	}()
}

func (chs *ChartChangeSync) DoChartChangeSync(ns []string) (bool, error) {
	var exist bool
	var newRev string
	var err error
	if exist, newRev, err = chs.newCommits(); err != nil {
		return false, fmt.Errorf("Failure during retrieving commits: %#v", err)
	}
	if !exist {
		return false, nil
	}

	chartDirs, err := getChartDirs(chs.logger, chs.release.Repo.ConfigSync)
	if err != nil {
		return false, fmt.Errorf("Failure to get charts under the charts path: %#v", err)
	}

	chartFhrs := make(map[string][]ifv1.FluxHelmRelease)
	for _, chart := range chartDirs {
		err = chs.getCustomResources(ns, chart, chartFhrs)
		if err != nil {
			return false, fmt.Errorf("Failure during retrieving Custom Resources related to Chart [%s]: %#v", chart, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
	chartsToRelease, err := chs.releaseNeeded(ctx, newRev, chartDirs, chartFhrs)
	cancel()
	if err != nil {
		return false, fmt.Errorf("Failure while establishing upgrade need of releases: %#v", err)
	}
	if len(chartsToRelease) == 0 {
		chs.lastCheckedRevision = newRev
		return false, nil
	}

	if err = chs.releaseCharts(chartsToRelease, chartFhrs); err != nil {
		return false, fmt.Errorf("Failure to release Chart(s): %#v", err)
	}

	return true, nil
}

// GetNamespaces gets current kubernetes cluster namespaces
func GetNamespaces(logger log.Logger, kubeClient kubernetes.Clientset) ([]string, error) {
	ns := []string{}

	nso, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		errm := fmt.Errorf("Failure while retrieving kubernetes namespaces: %#v", err)
		logger.Log("error", errm.Error())
		return nil, errm
	}

	for _, n := range nso.Items {
		ns = append(ns, n.GetName())
	}

	return ns, nil
}

// getChartDirs retrieves charts under the charts directory (under the repo root)
//		go-git(.v4) does not implement finding commits under a specific path. This means
//		the individual chart paths cannor be currently used with "git log"
func getChartDirs(logger log.Logger, checkout *helmgit.Checkout) ([]string, error) {
	chartDirs := []string{}

	repoRoot := checkout.Dir
	if repoRoot == "" {
		return nil, helmgit.ErrNoRepoCloned
	}
	chartsFullPath := filepath.Join(repoRoot, checkout.Config.Path)

	files, err := ioutil.ReadDir(chartsFullPath)
	if err != nil {
		errm := fmt.Errorf("Failure to access directory %s: %#v", chartsFullPath, err)
		logger.Log("error", errm.Error())
		return nil, errm
	}

	// We only choose subdirectories that represent Charts
	for _, f := range files {
		if f.IsDir() {
			chartDir := filepath.Join(chartsFullPath, f.Name())
			chartMeta := filepath.Join(chartDir, "Chart.yaml")
			if _, err := os.Stat(chartMeta); os.IsNotExist(err) {
				continue
			}
			chartDirs = append(chartDirs, f.Name())
		}
	}

	return chartDirs, nil
}

// newCommits determines if charts need to be released
//		go-git.v4 does not provide a possibility to find commit for a particular path.
// 		So we find if there are any commits at all sice last time
func (chs *ChartChangeSync) newCommits() (bool, string, error) {
	chs.logger.Log("info", "Getting new commits")

	checkout := chs.release.Repo.ConfigSync

	chs.logger.Log("info", fmt.Sprintf("Repo dir = %s", checkout.Dir))

	if checkout.Dir == "" {
		ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
		err := checkout.Clone(ctx, helmgit.ChangesClone)
		cancel()
		if err != nil {
			errm := fmt.Errorf("Failure while cloning repo : %#v", err)
			chs.logger.Log("error", errm.Error())
			return false, "", errm
		}
		chs.logger.Log("info", "Cloned repo")
	}

	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultPullTimeout)
	err := checkout.Pull(ctx)
	cancel()
	if err != nil {
		return false, "", fmt.Errorf("Failure while pulling repo: %#v", err)
	}

	// get latest revision
	newRev, err := checkout.GetRevision()
	if err != nil {
		return false, "", fmt.Errorf("Failure while getting repo revision: %s", err.Error())
	}
	chs.logger.Log("info", fmt.Sprintf("Got revision %s", newRev.String()))

	oldRev := chs.lastCheckedRevision
	if oldRev == "" {
		chs.lastCheckedRevision = newRev.String()
		chs.logger.Log("debug", fmt.Sprintf("Populated lastCheckedRevision with %s", chs.lastCheckedRevision))

		return false, "", nil
	}

	chs.logger.Log("info", fmt.Sprintf("lastCheckedRevision: %s", chs.lastCheckedRevision))

	if oldRev != newRev.String() {
		return true, newRev.String(), nil
	}
	return false, "", nil
}

// getCustomResources assembles custom resources referencing a particular chart
func (chs *ChartChangeSync) getCustomResources(namespaces []string, chart string, chartFhrs map[string][]ifv1.FluxHelmRelease) error {
	chartSelector := map[string]string{
		"chart": chart,
	}
	labelsSet := labels.Set(chartSelector)
	listOptions := metav1.ListOptions{LabelSelector: labelsSet.AsSelector().String()}

	fhrs := []ifv1.FluxHelmRelease{}
	for _, ns := range namespaces {
		list, err := chs.ifClient.HelmV1alpha2().FluxHelmReleases(ns).List(listOptions)
		if err != nil {
			chs.logger.Log("error", fmt.Errorf("Failure while retrieving FluxHelmReleases: %#v", err))
			continue
		}

		for _, fhr := range list.Items {
			fhrs = append(fhrs, fhr)
		}
	}

	chartFhrs[chart] = fhrs

	return nil
}

// releaseCharts upgrades releases with changed Charts
//		input:
//					chartD ... provides chart name and its directory information
//					fhr ...... provides chart name and all Custom Resources associated with this chart
func (chs *ChartChangeSync) releaseCharts(chartsToRelease []string, chartFhrs map[string][]ifv1.FluxHelmRelease) error {
	checkout := chs.release.Repo.ConfigSync

	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultPullTimeout)
	err := checkout.Pull(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("Failure while pulling repo: %#v", err)
	}

	chartPathBase := filepath.Join(checkout.Dir, checkout.Config.Path)
	for _, chart := range chartsToRelease {
		var err error
		fhrs := chartFhrs[chart]
		for _, fhr := range fhrs {

			// sanity check
			chartPath := filepath.Join(chartPathBase, fhr.Spec.ChartGitPath)
			if _, err := os.Stat(chartPath); os.IsNotExist(err) {
				chs.logger.Log("error", fmt.Sprintf("Missing Chart %s. No release can happen.", chartPath))
				continue
			}

			rlsName := chartrelease.GetReleaseName(fhr)

			opts := chartrelease.InstallOptions{DryRun: false}
			_, err = chs.release.Install(checkout, rlsName, fhr, chartrelease.UpgradeAction, opts)
			if err != nil {
				chs.logger.Log("info", fmt.Sprintf("Error to do upgrade of release of [%s]: %s. Skipping.", rlsName, err.Error()))
				// TODO: collect errors and return them after looping through all - ?
				continue
			}
			chs.logger.Log("info", fmt.Sprintf("Release [%s] upgraded due to chart only changes", rlsName))
		}
	}

	// get latest revision
	newRev, err := checkout.GetRevision()
	if err != nil {
		return fmt.Errorf("Failure while getting repo revision: %s", err.Error())
	}
	chs.lastCheckedRevision = newRev.String()
	chs.logger.Log("debug", fmt.Sprintf("Populated lastCheckedRevision with %s", chs.lastCheckedRevision))

	return nil
}

// releaseNeeded finds if there were commits related to Chart changes since last sync
//	returns maps keys on chart name with value corresponding to the chart path
// (go-git.v4 does not provide a possibility to find commit for a particular path.)
func (chs *ChartChangeSync) releaseNeeded(ctx context.Context, newRev string, charts []string, chartFhrs map[string][]ifv1.FluxHelmRelease) ([]string, error) {
	chartsToRelease := []string{}
	var changed, ok bool
	var err error
	var fhrs []ifv1.FluxHelmRelease

	revRange := fmt.Sprintf("%s..%s", chs.lastCheckedRevision, newRev)
	dir := fmt.Sprintf("%s/%s", chs.release.Repo.ConfigSync.Dir, chs.release.Repo.ConfigSync.Config.Path)

	for _, chart := range charts {
		chs.logger.Log("debug", fmt.Sprintf("Testing if release needed for Chart [%s]", chart))

		if fhrs, ok = chartFhrs[chart]; !ok {
			continue
		}
		if len(fhrs) < 1 {
			continue
		}

		if changed, err = chs.chartChanged(ctx, dir, revRange, chart); err != nil {
			chs.logger.Log("error", fmt.Sprintf("Failure to determine chart change for [%s]: %s", chart, err.Error()))
			continue
		}
		if !changed {
			continue
		}
		chartsToRelease = append(chartsToRelease, chart)
	}
	return chartsToRelease, nil
}
