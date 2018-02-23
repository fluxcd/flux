/*
Package sync provides the functionality for updating a Chart release
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

	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"

	//"gopkg.in/src-d/go-git.v4/plumbing"

	ifv1 "github.com/weaveworks/flux/apis/integrations.flux/v1"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	iflister "github.com/weaveworks/flux/integrations/client/listers/integrations.flux/v1" // kubernetes 1.9
	//iflister "github.com/weaveworks/flux/integrations/client/listers/integrations/v1" // kubernetes 1.8
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
	release           *chartrelease.Release
	kubeClient        kubernetes.Clientset
	ifClient          ifclientset.Clientset
	fhrLister         iflister.FluxHelmResourceLister
	lastCheckedCommit plumbing.Hash
	sync.RWMutex
}

//  Run ... create a syncing loop
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error) {
	chs.release.Repo.ChartsSync.Lock()
	defer chs.release.Repo.ChartsSync.Unlock()
	defer runtime.HandleCrash()

	go func() {
		ticker := time.NewTicker(chs.Polling.Interval)
		defer ticker.Stop()
		defer chs.release.Repo.ChartsSync.Cleanup()

		var exist bool
		var err error

		for {
			select {
			case <-ticker.C:
				// new commits?
				if exist, err = chs.newCommits(); err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure during retrieving commits: %#v", err))
					continue
				}
				// continue if not
				if !exist {
					continue
				}
				// get namespaces
				ns, err := GetNamespaces(chs.logger, chs.kubeClient)
				if err != nil {
					errc <- err
				}
				// get chart dirs
				chartDirs, err := chs.getChartDirs()
				if err != nil {
					continue
				}
				// get fhrs
				chartFhrs := make(map[string][]ifv1.FluxHelmResource)
				for chart := range chartDirs {
					err = chs.getCustomResources(ns, chart, chartFhrs)
					if err != nil {
						chs.logger.Log("error", fmt.Sprintf("Failure during retrieving Custom Resources related to Chart: %s", chart))
						continue
					}
				}
				if len(chartFhrs) < 1 {
					continue
				}
				// compare manifests and release if required
				if err = chs.releaseCharts(chartDirs, chartFhrs); err != nil {
					chs.logger.Log("error", fmt.Sprintf("%#v", err))
				}
			case <-stopCh:
				break
			}
		}

	}()

	<-stopCh
}

// GetNamespaces ... gets current kubernetes cluster namespaces
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

// getChartDirs ... retrieves charts under the charts directory (under the repo root)
func (chs *ChartChangeSync) getChartDirs() (map[string]string, error) {
	chartD := make(map[string]string)

	checkout := chs.release.Repo.ChartsSync
	repoRoot := checkout.Dir
	if repoRoot == "" {
		return nil, helmgit.ErrNoRepoCloned
	}
	chartsFullPath := filepath.Join(repoRoot, checkout.Config.Path)

	files, err := ioutil.ReadDir(chartsFullPath)
	if err != nil {
		errm := fmt.Errorf("Failure to access directory %s: %#v", chartsFullPath, err)
		chs.logger.Log("error", errm.Error())
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
			chartD[f.Name()] = chartDir
		}
	}

	return chartD, nil
}

// newCommits ... determines if charts need to be released
//		go-git.v4 does not provide a possibility to find commit for a particular path.
// 		So we find if there are any commits at all sice last time
func (chs *ChartChangeSync) newCommits() (bool, error) {
	chs.Lock()
	defer chs.Unlock()

	checkout := chs.release.Repo.ChartsSync

	if checkout.Dir == "" {
		ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultCloneTimeout)
		err := checkout.Clone(ctx, helmgit.ChartsChangesClone)
		cancel()
		if err != nil {
			errm := fmt.Errorf("Failure while cloning repo : %#v", err)
			chs.logger.Log("error", errm.Error())
			return false, errm
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), helmgit.DefaultPullTimeout)
	err := checkout.Pull(ctx)
	cancel()
	if err != nil {
		errm := fmt.Errorf("Failure while pulling repo: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, errm
	}
	// get latest revision
	ctx, cancel = context.WithTimeout(context.Background(), helmgit.DefaultPullTimeout)
	newRev, err := checkout.GetRevision()
	cancel()
	if err != nil {
		errm := fmt.Errorf("Failure while getting repo revision: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, errm
	}

	// if lastCheckedCommit field is missing then all charts need to be assessed
	oldRev := chs.lastCheckedCommit
	if oldRev.String() == "" {
		chs.lastCheckedCommit = newRev

		return true, nil
	}

	// go-git.v4 does not provide a possibility to find commit for a particular path.
	// So we find if there are any commits at all sice last time (since oldRev)
	commitIter, err := checkout.Repo.Log(&git.LogOptions{From: oldRev})
	if err != nil {
		errm := fmt.Errorf("Failure while getting commit info: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, errm
	}
	var count int
	err = commitIter.ForEach(func(c *object.Commit) error {
		count = count + 1
		return nil
	})
	if err != nil {
		errm := fmt.Errorf("Failure while getting commit info: %#v", err)
		chs.logger.Log("error", errm.Error())
		return false, errm
	}

	if count > 0 {
		return true, nil
	}
	return false, nil
}

// getCustomResources ... assembles custom resources referencing a particular chart
func (chs *ChartChangeSync) getCustomResources(namespaces []string, chart string, chartFhrs map[string][]ifv1.FluxHelmResource) error {
	fhrs := []ifv1.FluxHelmResource{}

	chartSelector := map[string]string{
		"chart": chart,
	}
	labelsSet := labels.Set(chartSelector)
	listOptions := metav1.ListOptions{LabelSelector: labelsSet.AsSelector().String()}

	for _, ns := range namespaces {
		list, err := chs.ifClient.IntegrationsV1().FluxHelmResources(ns).List(listOptions)
		if err != nil {
			chs.logger.Log("error", fmt.Errorf("Failure while retrieving FluxHelmResources: %#v", err))
			continue
		}

		for _, fhr := range list.Items {
			fmt.Printf("\t\t>>>  %v \n\n", fhr)
			fhrs = append(fhrs, fhr)
		}
	}
	chartFhrs[chart] = fhrs

	return nil
}

// releaseCharts ... release a Chart if required
//		input:
//					chartD ... provides chart name and its directory information
//					fhr ...... provides chart name and all Custom Resources associated with this chart
//		does a dry run and compares the manifests (and value file?) If differences => release)
func (chs *ChartChangeSync) releaseCharts(chartDirs map[string]string, chartFhrs map[string][]ifv1.FluxHelmResource) error {
	helmCl := chs.release.HelmClient
	checkout := chs.release.Repo.ChartsSync
	for chart, _ := range chartDirs {
		fhrs := chartFhrs[chart]
		for _, fhr := range fhrs {
			rlsName := chartrelease.GetReleaseName(fhr)
			// get current release
			currRlsRes, err := helmCl.ReleaseContent(rlsName)
			if err != nil {
				chs.logger.Log("info", fmt.Sprintf("Getting release [%s] for upgrade due to charts changes: %s. Skipping.", rlsName, err.Error()))
				continue
			}
			// get dry-run of a new release
			newRls, err := chs.release.Install(checkout, rlsName, fhr, "UPDATE", true)
			if err != nil {
				chs.logger.Log("info", fmt.Sprintf("Error during dry run upgrade of release of [%s]: %s. Skipping.", rlsName, err.Error()))
				continue
			}
			// compare manifests => release if different
			currMnf := currRlsRes.Release.GetManifest()
			newMnf := newRls.GetManifest()

			if currMnf == newMnf {
				continue
			}
			_, err = chs.release.Install(checkout, rlsName, fhr, "UPDATE", false)
			if err != nil {
				chs.logger.Log("info", fmt.Sprintf("Error during dry run upgrade of release of [%s]: %s. Skipping.", rlsName, err.Error()))
				// TODO: collect errors and return them after looping through all - ?
				continue
			}
			chs.logger.Log("info", fmt.Sprintf("Release [%s] upgraded due to chart only changes", rlsName))
		}
	}
	//NEXT:

	return nil
}
