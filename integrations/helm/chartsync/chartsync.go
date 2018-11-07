/*

This package has the algorithm for making sure the Helm releases in
the cluster match what are defined in the HelmRelease resources.

There are several ways they can be mismatched. Here's how they are
reconciled:

 1a. There is a HelmRelease resource, but no corresponding
   release. This can happen when the helm operator is first run, for
   example. The ChartChangeSync periodically checks for this by
   running through the resources and installing any that aren't
   released already.

 1b. The release corresponding to a HelmRelease has been updated by
   some other means, perhaps while the operator wasn't running. This
   is also checked periodically, by doing a dry-run release and
   comparing the result to the release.

 2. The chart has changed in git, meaning the release is out of
   date. The ChartChangeSync responds to new git commits by looking at
   each chart that's referenced by a HelmRelease, and if it's
   changed since the last seen commit, updating the release.

1a.) and 1b.) run on the same schedule, and 2.) is run when a git
mirror reports it has fetched from upstream _and_ (upon checking) the
head of the branch has changed.

Since both 1*.) and 2.) look at the charts in the git repo, but run on
different schedules (non-deterministically), there's a chance that
they can fight each other. For example, the git mirror may fetch new
commits which are used in 1), then treated as changes subsequently by
2). To keep consistency between the two, the current revision of a
repo is used by 1), and advanced only by 2).

*/
package chartsync

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	google_protobuf "github.com/golang/protobuf/ptypes/any"
	"github.com/google/go-cmp/cmp"
	"github.com/ncabatoff/go-seq/seq"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/weaveworks/flux/git"
	fluxv1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	helmop "github.com/weaveworks/flux/integrations/helm"
	"github.com/weaveworks/flux/integrations/helm/release"
	"github.com/weaveworks/flux/integrations/helm/status"
)

const (
	// condition change reasons
	ReasonGitNotReady      = "GitRepoNotCloned"
	ReasonDownloadFailed   = "RepoFetchFailed"
	ReasonDownloaded       = "RepoChartInCache"
	ReasonInstallFailed    = "HelmInstallFailed"
	ReasonDependencyFailed = "UpdateDependencyFailed"
	ReasonUpgradeFailed    = "HelmUgradeFailed"
	ReasonCloned           = "GitRepoCloned"
	ReasonSuccess          = "HelmSuccess"
)

type Polling struct {
	Interval time.Duration
}

type Clients struct {
	KubeClient kubernetes.Clientset
	IfClient   ifclientset.Clientset
}

type Config struct {
	ChartCache string
	LogDiffs   bool
	UpdateDeps bool
}

func (c Config) WithDefaults() Config {
	if c.ChartCache == "" {
		c.ChartCache = "/tmp"
	}
	return c
}

// clone puts a local git clone together with its state (head
// revision), so we can keep track of when it needs to be updated.
type clone struct {
	export *git.Export
	head   string
}

type ChartChangeSync struct {
	logger log.Logger
	Polling
	kubeClient kubernetes.Clientset
	ifClient   ifclientset.Clientset
	release    *release.Release
	config     Config

	mirrors *git.Mirrors

	clonesMu sync.Mutex
	clones   map[string]clone
}

func New(logger log.Logger, polling Polling, clients Clients, release *release.Release, config Config) *ChartChangeSync {
	return &ChartChangeSync{
		logger:     logger,
		Polling:    polling,
		kubeClient: clients.KubeClient,
		ifClient:   clients.IfClient,
		release:    release,
		config:     config.WithDefaults(),
		mirrors:    git.NewMirrors(),
		clones:     make(map[string]clone),
	}
}

// Run creates a syncing loop that will reconcile differences between
// Helm releases in the cluster, what HelmRelease declare, and
// changes in the git repos mentioned by any HelmRelease.
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	chs.logger.Log("info", "Starting charts sync loop")
	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer func() {
			chs.mirrors.StopAllAndWait()
			wg.Done()
		}()

		ticker := time.NewTicker(chs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			case reposChanged := <-chs.mirrors.Changes():
				// TODO(michael): the inefficient way, for now, until
				// it's clear how to better optimalise it
				resources, err := chs.getCustomResources()
				if err != nil {
					chs.logger.Log("warning", "failed to get custom resources", "err", err)
					continue
				}
				for _, fhr := range resources {
					if fhr.Spec.ChartSource.GitChartSource == nil {
						continue
					}

					repoURL := fhr.Spec.ChartSource.GitChartSource.GitURL
					repoName := mirrorName(fhr.Spec.ChartSource.GitChartSource)

					if _, ok := reposChanged[repoName]; !ok {
						continue
					}

					repo, ok := chs.mirrors.Get(repoName)
					if !ok {
						// Then why .. did you say .. it had changed? It may have been removed. Add it back and let it signal again.
						chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git mirror missing; starting mirroring again")
						chs.logger.Log("warning", "mirrored git repo disappeared after signalling change", "repo", repoName)
						chs.maybeMirror(fhr)
						continue
					}

					status, err := repo.Status()
					if status != git.RepoReady {
						chs.logger.Log("info", "repo not ready yet, while attempting chart sync", "repo", repoURL, "status", string(status))
						// TODO(michael) log if there's a problem with the following?
						chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, err.Error())
						continue
					}

					ref := fhr.Spec.ChartSource.GitChartSource.RefOrDefault()
					path := fhr.Spec.ChartSource.GitChartSource.Path
					releaseName := release.GetReleaseName(fhr)

					ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
					refHead, err := repo.Revision(ctx, ref)
					cancel()
					if err != nil {
						chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
						chs.logger.Log("warning", "could not get revision for ref while checking for changes", "repo", repoURL, "ref", ref, "err", err)
						continue
					}

					// This FHR is using a git repo; and, it appears to have had commits since we last saw it.
					// Check explicitly whether we should update its clone.
					chs.clonesMu.Lock()
					cloneForChart, ok := chs.clones[releaseName]
					chs.clonesMu.Unlock()

					if ok { // found clone
						ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
						commits, err := repo.CommitsBetween(ctx, cloneForChart.head, refHead, path)
						cancel()
						if err != nil {
							chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
							chs.logger.Log("warning", "could not get revision for ref while checking for changes", "repo", repoURL, "ref", ref, "err", err)
							continue
						}
						ok = len(commits) == 0
					}

					if !ok { // didn't find clone, or it needs updating
						ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
						newClone, err := repo.Export(ctx, refHead)
						cancel()
						if err != nil {
							chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
							chs.logger.Log("warning", "could not clone from mirror while checking for changes", "repo", repoURL, "ref", ref, "err", err)
							continue
						}
						newCloneForChart := clone{head: refHead, export: newClone}
						chs.clonesMu.Lock()
						chs.clones[releaseName] = newCloneForChart
						chs.clonesMu.Unlock()
						if cloneForChart.export != nil {
							cloneForChart.export.Clean()
						}
					}

					chs.reconcileReleaseDef(fhr)
				}
			case <-ticker.C:
				// Re-release any chart releases that have apparently
				// changed in the cluster.
				chs.logger.Log("info", fmt.Sprint("Start of releasesync"))
				err := chs.reapplyReleaseDefs()
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do manual release sync: %s", err))
				}
				chs.logger.Log("info", fmt.Sprint("End of releasesync"))

			case <-stopCh:
				chs.logger.Log("stopping", "true")
				return
			}
		}
	}()
}

func mirrorName(chartSource *fluxv1beta1.GitChartSource) string {
	return chartSource.GitURL // TODO(michael) this will not always be the case; e.g., per namespace, per auth
}

// maybeMirror starts mirroring the repo needed by a HelmRelease,
// if necessary
func (chs *ChartChangeSync) maybeMirror(fhr fluxv1beta1.HelmRelease) {
	chartSource := fhr.Spec.ChartSource.GitChartSource
	if chartSource != nil {
		if ok := chs.mirrors.Mirror(mirrorName(chartSource), git.Remote{chartSource.GitURL}, git.ReadOnly); !ok {
			chs.logger.Log("info", "started mirroring repo", "repo", chartSource.GitURL)
		}
	}
}

// ReconcileReleaseDef asks the ChartChangeSync to examine the release
// associated with a HelmRelease, and install or upgrade the
// release if the chart it refers to has changed.
func (chs *ChartChangeSync) ReconcileReleaseDef(fhr fluxv1beta1.HelmRelease) {
	chs.reconcileReleaseDef(fhr)
}

// reconcileReleaseDef looks up the helm release associated with a
// HelmRelease resource, and either installs, upgrades, or does
// nothing, depending on the state (or absence) of the release.
func (chs *ChartChangeSync) reconcileReleaseDef(fhr fluxv1beta1.HelmRelease) {
	releaseName := release.GetReleaseName(fhr)

	// There's no exact way in the Helm API to test whether a release
	// exists or not. Instead, try to fetch it, and treat an error as
	// not existing (and possibly fail further below, if it meant
	// something else).
	rel, _ := chs.release.GetDeployedRelease(releaseName)

	opts := release.InstallOptions{DryRun: false}

	chartPath := ""
	if fhr.Spec.ChartSource.GitChartSource != nil {
		chartSource := fhr.Spec.ChartSource.GitChartSource
		// We need to hold the lock until after we're done releasing
		// the chart, so that the clone doesn't get swapped out from
		// under us. TODO(michael) consider having a lock per clone.
		chs.clonesMu.Lock()
		defer chs.clonesMu.Unlock()
		chartClone, ok := chs.clones[releaseName]
		// FIXME(michael): if it's not cloned, and it's not going to
		// be, we might not want to wait around until the next tick
		// before reporting what's wrong with it. But if we just use
		// repo.Ready(), we'll force all charts through that blocking
		// code, rather than waiting for things to sync in good time.
		if !ok {
			repo, ok := chs.mirrors.Get(mirrorName(chartSource))
			if !ok {
				chs.maybeMirror(fhr)
				chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git repo "+chartSource.GitURL+" not mirrored yet")
				chs.logger.Log("info", "chart repo not cloned yet", "releaseName", releaseName, "resource", fmt.Sprintf("%s:%s/%s", fhr.Namespace, fhr.Kind, fhr.Name))
			} else {
				status, err := repo.Status()
				if status != git.RepoReady {
					chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git repo not mirrored yet: "+err.Error())
					chs.logger.Log("info", "chart repo not ready yet", "releaseName", releaseName, "resource", fmt.Sprintf("%s:%s/%s", fhr.Namespace, fhr.Kind, fhr.Name), "status", string(status), "err", err)
				}
			}
			return
		}
		chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionTrue, ReasonCloned, "successfully cloned git repo")
		chartPath = filepath.Join(chartClone.export.Dir(), chartSource.Path)

		if chs.config.UpdateDeps {
			if err := updateDependencies(chartPath); err != nil {
				chs.setCondition(&fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonDependencyFailed, err.Error())
				chs.logger.Log("warning", "Failed to update chart dependencies", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
				return
			}
		}

	} else if fhr.Spec.ChartSource.RepoChartSource != nil { // TODO(michael): make this dispatch more natural, or factor it out
		chartSource := fhr.Spec.ChartSource.RepoChartSource
		path, err := ensureChartFetched(chs.config.ChartCache, chartSource)
		if err != nil {
			chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonDownloadFailed, "chart download failed: "+err.Error())
			chs.logger.Log("info", "chart download failed", "releaseName", releaseName, "resource", fhr.ResourceID().String(), "err", err)
			return
		}
		chs.setCondition(&fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionTrue, ReasonDownloaded, "chart fetched: "+filepath.Base(path))
		chartPath = path
	}

	if rel == nil {
		_, err := chs.release.Install(chartPath, releaseName, fhr, release.InstallAction, opts, &chs.kubeClient)
		if err != nil {
			chs.setCondition(&fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonInstallFailed, err.Error())
			chs.logger.Log("warning", "Failed to install chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
			return
		}
		chs.setCondition(&fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionTrue, ReasonSuccess, "helm install succeeded")
		return
	}

	changed, err := chs.shouldUpgrade(chartPath, rel, fhr)
	if err != nil {
		chs.logger.Log("warning", "Unable to determine if release has changed", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
		return
	}
	if changed {
		_, err := chs.release.Install(chartPath, releaseName, fhr, release.UpgradeAction, opts, &chs.kubeClient)
		if err != nil {
			chs.setCondition(&fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonUpgradeFailed, err.Error())
			chs.logger.Log("warning", "Failed to upgrade chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
			return
		}
		chs.setCondition(&fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionTrue, ReasonSuccess, "helm upgrade succeeded")
	}
}

// reapplyReleaseDefs goes through the resource definitions and
// reconciles them with Helm releases. This is a "backstop" for the
// other sync processes, to cover the case of a release being changed
// out-of-band (e.g., by someone using `helm upgrade`).
func (chs *ChartChangeSync) reapplyReleaseDefs() error {
	resources, err := chs.getCustomResources()
	if err != nil {
		return fmt.Errorf("failed to get HelmRelease resources from the API server: %s", err.Error())
	}

	for _, fhr := range resources {
		chs.reconcileReleaseDef(fhr)
	}
	return nil
}

// DeleteRelease deletes the helm release associated with a
// HelmRelease. This exists mainly so that the operator code can
// call it when it is handling a resource deletion.
func (chs *ChartChangeSync) DeleteRelease(fhr fluxv1beta1.HelmRelease) {
	// FIXME(michael): these may need to stop mirroring a repo.
	name := release.GetReleaseName(fhr)
	err := chs.release.Delete(name)
	if err != nil {
		chs.logger.Log("warning", "Chart release not deleted", "release", name, "error", err)
	}
}

// ---

// getNamespaces gets current kubernetes cluster namespaces
func (chs *ChartChangeSync) getNamespaces() ([]string, error) {
	var ns []string
	nso, err := chs.kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failure while retrieving kubernetes namespaces: %s", err)
	}

	for _, n := range nso.Items {
		ns = append(ns, n.GetName())
	}

	return ns, nil
}

// getCustomResources assembles all custom resources in all namespaces
func (chs *ChartChangeSync) getCustomResources() ([]fluxv1beta1.HelmRelease, error) {
	namespaces, err := chs.getNamespaces()
	if err != nil {
		return nil, err
	}

	var fhrs []fluxv1beta1.HelmRelease
	for _, ns := range namespaces {
		list, err := chs.ifClient.FluxV1beta1().HelmReleases(ns).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, fhr := range list.Items {
			fhrs = append(fhrs, fhr)
		}
	}
	return fhrs, nil
}

// setCondition saves the status of a condition, if it's new
// information. New information is something that adds or changes the
// status, reason or message (i.e., anything but the transition time)
// for one of the types of condition.
func (chs *ChartChangeSync) setCondition(fhr *fluxv1beta1.HelmRelease, typ fluxv1beta1.HelmReleaseConditionType, st v1.ConditionStatus, reason, message string) error {
	for _, c := range fhr.Status.Conditions {
		if c.Type == typ && c.Status == st && c.Message == message && c.Reason == reason {
			return nil
		}
	}

	fhrClient := chs.ifClient.FluxV1beta1().HelmReleases(fhr.Namespace)
	cond := fluxv1beta1.HelmReleaseCondition{
		Type:               typ,
		Status:             st,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
	return status.UpdateConditions(fhrClient, fhr, cond)
}

func sortStrings(ss []string) []string {
	ret := append([]string{}, ss...)
	sort.Strings(ret)
	return ret
}

func sortChartFields(c *hapi_chart.Chart) *hapi_chart.Chart {
	nc := hapi_chart.Chart{
		Metadata:  &(*c.Metadata),
		Templates: append([]*hapi_chart.Template{}, c.Templates...),
		Files:     append([]*google_protobuf.Any{}, c.Files...),
	}

	if c.Values != nil {
		nc.Values = &(*c.Values)
	}

	sort.SliceStable(nc.Files, func(i, j int) bool {
		return seq.Compare(nc.Files[i], nc.Files[j]) < 0
	})
	sort.SliceStable(nc.Templates, func(i, j int) bool {
		return seq.Compare(nc.Templates[i], nc.Templates[j]) < 0
	})

	nc.Metadata.Sources = sortStrings(nc.Metadata.Sources)
	nc.Metadata.Keywords = sortStrings(nc.Metadata.Keywords)
	nc.Metadata.Maintainers = append([]*hapi_chart.Maintainer{}, nc.Metadata.Maintainers...)
	sort.SliceStable(nc.Metadata.Maintainers, func(i, j int) bool {
		return seq.Compare(nc.Metadata.Maintainers[i], nc.Metadata.Maintainers[j]) < 0
	})

	nc.Dependencies = make([]*hapi_chart.Chart, len(c.Dependencies))
	for i := range c.Dependencies {
		nc.Dependencies[i] = sortChartFields(c.Dependencies[i])
	}
	sort.SliceStable(nc.Dependencies, func(i, j int) bool {
		return seq.Compare(nc.Dependencies[i], nc.Dependencies[j]) < 0
	})

	return &nc
}

// shouldUpgrade returns true if the current running values or chart
// don't match what the repo says we ought to be running, based on
// doing a dry run install from the chart in the git repo.
func (chs *ChartChangeSync) shouldUpgrade(chartsRepo string, currRel *hapi_release.Release, fhr fluxv1beta1.HelmRelease) (bool, error) {
	if currRel == nil {
		return false, fmt.Errorf("No Chart release provided for %v", fhr.GetName())
	}

	currVals := currRel.GetConfig()
	currChart := currRel.GetChart()

	// Get the desired release state
	opts := release.InstallOptions{DryRun: true}
	tempRelName := currRel.GetName() + "-temp"
	desRel, err := chs.release.Install(chartsRepo, tempRelName, fhr, release.InstallAction, opts, &chs.kubeClient)
	if err != nil {
		return false, err
	}
	desVals := desRel.GetConfig()
	desChart := desRel.GetChart()

	// compare values && Chart
	if diff := cmp.Diff(currVals, desVals); diff != "" {
		if chs.config.LogDiffs {
			chs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()))
		}
		return true, nil
	}

	if diff := cmp.Diff(sortChartFields(currChart), sortChartFields(desChart)); diff != "" {
		if chs.config.LogDiffs {
			chs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()))
		}
		return true, nil
	}

	return false, nil
}
