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

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	fhrv1 "github.com/weaveworks/flux/integrations/client/informers/externalversions/helm.integrations.flux.weave.works/v1alpha"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type Polling struct {
	Interval time.Duration
	Timeout  time.Duration
}

type ChartChangeSync struct {
	logger log.Logger
	Polling
	kubeClient          kubernetes.Clientset
	ifClient            ifclientset.Clientset
	release             *chartrelease.Release
	lastCheckedRevision string
}

func New(
	logger log.Logger, syncInterval time.Duration, syncTimeout time.Duration,
	kubeClient kubernetes.Clientset,
	ifClient ifclientset.Clientset, fhrInformer fhrv1.FluxHelmReleaseInformer,
	release *chartrelease.Release) *ChartChangeSync {

	lastCheckedRevision := ""
	gitRef, err := release.Repo.ConfigSync.GetRevision()
	if err != nil {
		// we shall try again later
	}
	lastCheckedRevision = gitRef.String()

	return &ChartChangeSync{
		logger:              logger,
		Polling:             Polling{Interval: syncInterval, Timeout: syncTimeout},
		kubeClient:          kubeClient,
		ifClient:            ifClient,
		release:             release,
		lastCheckedRevision: lastCheckedRevision,
	}
}

//  Run creates a syncing loop monitoring repo chart changes
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	chs.logger.Log("info", "Starting repo charts sync loop")

	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer wg.Done()

		defer chs.release.Repo.ChartsSync.Cleanup()

		var exist bool
		var newRev string
		var err error

		ticker := time.NewTicker(chs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				chs.logger.Log("info", fmt.Sprint("Start of chartsync"))
				// new commits?
				if exist, newRev, err = chs.newCommits(); err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure during retrieving commits: %#v", err))
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}
				if !exist {
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}

				ns, err := GetNamespaces(chs.logger, chs.kubeClient)
				if err != nil {
					errc <- err
				}
				chartDirs, err := getChartDirs(chs.logger, chs.release.Repo.ChartsSync)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to get charts under the charts path: %#v", err))
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}
				// get fhrs
				chartFhrs := make(map[string][]ifv1.FluxHelmRelease)
				for _, chart := range chartDirs {
					err = chs.getCustomResources(ns, chart, chartFhrs)
					if err != nil {
						chs.logger.Log("error", fmt.Sprintf("Failure during retrieving Custom Resources related to Chart [%s]: %#v", chart, err))
						chs.logger.Log("info", fmt.Sprint("End of chartsync"))
						continue
					}
				}

				// compare manifests and release if required
				ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				chartsToRelease, err := chs.releaseNeeded(ctx, newRev, chartDirs, chartFhrs)
				cancel()
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Error while establishing upgrade need of releases: %s", err.Error()))
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}
				// Nothing to release
				if len(chartsToRelease) == 0 {
					chs.lastCheckedRevision = newRev
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}

				if err = chs.releaseCharts(chartsToRelease, chartFhrs); err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to release Chart(s): %#v", err))
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}
				// All went well, so we shall make the repo with the last checked commit up to date
				// and update the lastCheckedRevision property
				chc := chs.release.Repo.ChartsSync
				ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
				err = chc.Pull(ctx)
				cancel()
				if err != nil {
					errm := fmt.Errorf("Failure while pulling repo: %#v", err)
					chs.logger.Log("error", errm.Error())
					chs.logger.Log("info", fmt.Sprint("End of chartsync"))
					continue
				}
				chs.logger.Log("info", "Pulled repo")
				chs.lastCheckedRevision = newRev
				chs.logger.Log("info", fmt.Sprint("End of chartsync"))
			case <-stopCh:
				chs.logger.Log("stopping", "true")
				break
			}
		}
	}()
}

// GetNamespaces gets current kubernetes cluster namespaces
func GetNamespaces(logger log.Logger, kubeClient kubernetes.Clientset) ([]string, error) {
	ns := []string{}

	nso, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		errm := fmt.Errorf("Failure while retrieving kybernetes namespaces: %#v", err)
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

	checkout := chs.release.Repo.ChartsSync

	chs.logger.Log("info", fmt.Sprintf("Repo dir = %s", checkout.Dir))

	if checkout.Dir == "" {
		ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
		err := checkout.Clone(ctx, helmgit.ChartsChangesClone)
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
		errm := fmt.Errorf("Failure while pulling repo: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, "", errm
	}
	chs.logger.Log("info", "Pulled repo")

	// get latest revision
	newRev, err := checkout.GetRevision()
	if err != nil {
		errm := fmt.Errorf("Failure while getting repo revision: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, "", errm
	}
	chs.logger.Log("info", fmt.Sprintf("Got revision %s", newRev.String()))

	oldRev := chs.lastCheckedRevision
	if oldRev == "" {
		chs.lastCheckedRevision = newRev.String()
		chs.logger.Log("debug", fmt.Sprintf("Populated lastCheckedRevision: %s", chs.lastCheckedRevision))

		return false, "", nil
	}

	chs.logger.Log("info", fmt.Sprintf("lastCheckedRevision: %s", chs.lastCheckedRevision))
	chs.logger.Log("info", fmt.Sprintf("newRev: %s", newRev))

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
		list, err := chs.ifClient.HelmV1alpha().FluxHelmReleases(ns).List(listOptions)
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

// releaseCharts release a Chart if required
//		input:
//					chartD ... provides chart name and its directory information
//					fhr ...... provides chart name and all Custom Resources associated with this chart
//		does a dry run and compares the manifests (and value file?) If differences => release)
func (chs *ChartChangeSync) releaseCharts(chartsToRelease []string, chartFhrs map[string][]ifv1.FluxHelmRelease) error {
	checkout := chs.release.Repo.ChartsSync

	for _, chart := range chartsToRelease {
		var err error
		fhrs := chartFhrs[chart]
		for _, fhr := range fhrs {
			rlsName := chartrelease.GetReleaseName(fhr)

			opts := chartrelease.InstallOptions{DryRun: false}
			_, err = chs.release.Install(checkout, rlsName, fhr, "UPDATE", opts)
			if err != nil {
				chs.logger.Log("info", fmt.Sprintf("Error during dry run upgrade of release of [%s]: %s. Skipping.", rlsName, err.Error()))
				// TODO: collect errors and return them after looping through all - ?
				continue
			}
			chs.logger.Log("info", fmt.Sprintf("Release [%s] upgraded due to chart only changes", rlsName))
		}
	}

	return nil
}

// releaseNeeded finds if there were commits in the repo since the last charts sync
//	returns maps keys on chart name with value corresponding to the chart path
// (go-git.v4 does not provide a possibility to find commit for a particular path.)
func (chs *ChartChangeSync) releaseNeeded(ctx context.Context, newRev string, charts []string, chartFhrs map[string][]ifv1.FluxHelmRelease) ([]string, error) {
	chartsToRelease := []string{}
	var changed, ok bool
	var err error
	var fhrs []ifv1.FluxHelmRelease

	revRange := fmt.Sprintf("%s..%s", chs.lastCheckedRevision, newRev)
	dir := fmt.Sprintf("%s/%s", chs.release.Repo.ChartsSync.Dir, chs.release.Repo.ChartsSync.Config.Path)

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
