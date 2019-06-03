/*

This package has the algorithm for making sure the Helm releases in
the cluster match what are defined in the HelmRelease resources.

There are several ways they can be mismatched. Here's how they are
reconciled:

 1a. There is a HelmRelease resource, but no corresponding
   release. This can happen when the helm operator is first run, for
   example.

 1b. The release corresponding to a HelmRelease has been updated by
   some other means, perhaps while the operator wasn't running. This
   is also checked, by doing a dry-run release and comparing the result
   to the release.

 2. The chart has changed in git, meaning the release is out of
   date. The ChartChangeSync responds to new git commits by looking up
   each chart that makes use of the mirror that has new commits,
   replacing the clone for that chart, and scheduling a new release.

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

	"k8s.io/apimachinery/pkg/labels"

	"github.com/go-kit/kit/log"
	google_protobuf "github.com/golang/protobuf/ptypes/any"
	"github.com/google/go-cmp/cmp"
	"github.com/ncabatoff/go-seq/seq"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	fluxv1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	iflister "github.com/weaveworks/flux/integrations/client/listers/flux.weave.works/v1beta1"
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

type Clients struct {
	KubeClient kubernetes.Clientset
	IfClient   ifclientset.Clientset
	FhrLister  iflister.HelmReleaseLister
}

type Config struct {
	ChartCache      string
	LogDiffs        bool
	UpdateDeps      bool
	GitTimeout      time.Duration
	GitPollInterval time.Duration
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
	remote string
	ref    string
	head   string
}

// ReleaseQueue is an add-only workqueue.RateLimitingInterface
type ReleaseQueue interface {
	AddRateLimited(item interface{})
}

type ChartChangeSync struct {
	logger       log.Logger
	kubeClient   kubernetes.Clientset
	ifClient     ifclientset.Clientset
	fhrLister    iflister.HelmReleaseLister
	release      *release.Release
	releaseQueue ReleaseQueue
	config       Config

	mirrors *git.Mirrors

	clonesMu sync.Mutex
	clones   map[string]clone

	namespace string
}

func New(logger log.Logger, clients Clients, release *release.Release, releaseQueue ReleaseQueue, config Config, namespace string) *ChartChangeSync {
	return &ChartChangeSync{
		logger:       logger,
		kubeClient:   clients.KubeClient,
		ifClient:     clients.IfClient,
		fhrLister:    clients.FhrLister,
		release:      release,
		releaseQueue: releaseQueue,
		config:       config.WithDefaults(),
		mirrors:      git.NewMirrors(),
		clones:       make(map[string]clone),
		namespace:    namespace,
	}
}

// Run creates a syncing loop that will reconcile differences between
// Helm releases in the cluster, what HelmRelease declare, and
// changes in the git repos mentioned by any HelmRelease.
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	chs.logger.Log("info", "starting git chart sync loop")

	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer func() {
			chs.mirrors.StopAllAndWait()
			wg.Done()
		}()

		for {
			select {
			case mirrorsChanged := <-chs.mirrors.Changes():
				for mirror := range mirrorsChanged {
					resources, err := chs.getCustomResourcesForMirror(mirror)
					if err != nil {
						chs.logger.Log("warning", "failed to get custom resources", "err", err)
						continue
					}

					// Retrieve the mirror we got a change signal for
					repo, ok := chs.mirrors.Get(mirror)
					if !ok {
						// Then why .. did you say .. it had changed? It may have been removed. Add it back and let it signal again.
						chs.logger.Log("warning", "mirrored git repo disappeared after signalling change", "repo", mirror)
						for _, fhr := range resources {
							chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git mirror missing; starting mirroring again")
							chs.maybeMirror(fhr)
						}
						continue
					}

					// Ensure the repo is ready
					status, err := repo.Status()
					if status != git.RepoReady {
						chs.logger.Log("info", "repo not ready yet, while attempting chart sync", "repo", mirror, "status", string(status))
						for _, fhr := range resources {
							// TODO(michael) log if there's a problem with the following?
							chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, err.Error())
						}
						continue
					}

					// Determine if we need to update the clone and
					// schedule an upgrade for every HelmRelease that
					// makes use of the mirror
					for _, fhr := range resources {
						ref := fhr.Spec.ChartSource.GitChartSource.RefOrDefault()
						path := fhr.Spec.ChartSource.GitChartSource.Path
						releaseName := release.GetReleaseName(fhr)

						ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
						refHead, err := repo.Revision(ctx, ref)
						cancel()
						if err != nil {
							chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
							chs.logger.Log("warning", "could not get revision for ref while checking for changes", "resource", fhr.ResourceID().String(), "repo", mirror, "ref", ref, "err", err)
							continue
						}

						// The git repo of this appears to have had commits since we last saw it,
						// check explicitly whether we should update its clone.
						chs.clonesMu.Lock()
						cloneForChart, ok := chs.clones[releaseName]
						chs.clonesMu.Unlock()

						if ok { // found clone
							ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
							commits, err := repo.CommitsBetween(ctx, cloneForChart.head, refHead, path)
							cancel()
							if err != nil {
								chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
								chs.logger.Log("warning", "could not get revision for ref while checking for changes", "resource", fhr.ResourceID().String(), "repo", mirror, "ref", ref, "err", err)
								continue
							}
							ok = len(commits) == 0
						}

						if !ok { // didn't find clone, or it needs updating
							ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
							newClone, err := repo.Export(ctx, refHead)
							cancel()
							if err != nil {
								chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonGitNotReady, "problem cloning from local git mirror: "+err.Error())
								chs.logger.Log("warning", "could not clone from mirror while checking for changes", "resource", fhr.ResourceID().String(), "repo", mirror, "ref", ref, "err", err)
								continue
							}
							newCloneForChart := clone{remote: mirror, ref: ref, head: refHead, export: newClone}
							chs.clonesMu.Lock()
							chs.clones[releaseName] = newCloneForChart
							chs.clonesMu.Unlock()
							if cloneForChart.export != nil {
								cloneForChart.export.Clean()
							}

							// we have a (new) clone, enqueue a release
							cacheKey, err := cache.MetaNamespaceKeyFunc(fhr.GetObjectMeta())
							if err != nil {
								continue
							}
							chs.logger.Log("info", "enqueing release upgrade due to change in git chart source", "resource", fhr.ResourceID().String())
							chs.releaseQueue.AddRateLimited(cacheKey)
						}
					}
				}
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
		if ok := chs.mirrors.Mirror(
			mirrorName(chartSource),
			git.Remote{chartSource.GitURL}, git.Timeout(chs.config.GitTimeout), git.PollInterval(chs.config.GitPollInterval), git.ReadOnly,
		); !ok {
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

	// Attempt to retrieve an upgradable release, in case no release
	// or error is returned, install it.
	rel, err := chs.release.GetUpgradableRelease(releaseName)
	if err != nil {
		chs.logger.Log("warning", "unable to proceed with release", "resource", fhr.ResourceID().String(), "release", releaseName, "err", err)
		return
	}

	opts := release.InstallOptions{DryRun: false}

	chartPath := ""
	chartRevision := ""
	if fhr.Spec.ChartSource.GitChartSource != nil {
		chartSource := fhr.Spec.ChartSource.GitChartSource
		// We need to hold the lock until after we're done releasing
		// the chart, so that the clone doesn't get swapped out from
		// under us. TODO(michael) consider having a lock per clone.
		chs.clonesMu.Lock()
		defer chs.clonesMu.Unlock()
		chartClone, ok := chs.clones[releaseName]
		// Validate the clone we have for the release is the same as
		// is being referenced in the chart source.
		if ok {
			ok = chartClone.remote == chartSource.GitURL && chartClone.ref == chartSource.RefOrDefault()
			if !ok {
				if chartClone.export != nil {
					chartClone.export.Clean()
				}
				delete(chs.clones, releaseName)
			}
		}
		// FIXME(michael): if it's not cloned, and it's not going to
		// be, we might not want to wait around until the next tick
		// before reporting what's wrong with it. But if we just use
		// repo.Ready(), we'll force all charts through that blocking
		// code, rather than waiting for things to sync in good time.
		if !ok {
			repo, ok := chs.mirrors.Get(mirrorName(chartSource))
			if !ok {
				chs.maybeMirror(fhr)
				chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git repo "+chartSource.GitURL+" not mirrored yet")
				chs.logger.Log("info", "chart repo not cloned yet", "resource", fhr.ResourceID().String())
			} else {
				status, err := repo.Status()
				if status != git.RepoReady {
					chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionUnknown, ReasonGitNotReady, "git repo not mirrored yet: "+err.Error())
					chs.logger.Log("info", "chart repo not ready yet", "resource", fhr.ResourceID().String(), "status", string(status), "err", err)
				}
			}
			return
		}
		chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionTrue, ReasonCloned, "successfully cloned git repo")
		chartPath = filepath.Join(chartClone.export.Dir(), chartSource.Path)
		chartRevision = chartClone.head

		if chs.config.UpdateDeps && !fhr.Spec.ChartSource.GitChartSource.SkipDepUpdate {
			if err := updateDependencies(chartPath, ""); err != nil {
				chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonDependencyFailed, err.Error())
				chs.logger.Log("warning", "failed to update chart dependencies", "resource", fhr.ResourceID().String(), "err", err)
				return
			}
		}
	} else if fhr.Spec.ChartSource.RepoChartSource != nil { // TODO(michael): make this dispatch more natural, or factor it out
		chartSource := fhr.Spec.ChartSource.RepoChartSource
		path, err := ensureChartFetched(chs.config.ChartCache, chartSource)
		if err != nil {
			chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionFalse, ReasonDownloadFailed, "chart download failed: "+err.Error())
			chs.logger.Log("info", "chart download failed", "resource", fhr.ResourceID().String(), "err", err)
			return
		}
		chs.setCondition(fhr, fluxv1beta1.HelmReleaseChartFetched, v1.ConditionTrue, ReasonDownloaded, "chart fetched: "+filepath.Base(path))
		chartPath = path
		chartRevision = chartSource.Version
	}

	if rel == nil {
		_, err := chs.release.Install(chartPath, releaseName, fhr, release.InstallAction, opts, &chs.kubeClient)
		if err != nil {
			chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonInstallFailed, err.Error())
			chs.logger.Log("warning", "failed to install chart", "resource", fhr.ResourceID().String(), "err", err)
			return
		}
		chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionTrue, ReasonSuccess, "helm install succeeded")
		if err = status.UpdateReleaseRevision(chs.ifClient.FluxV1beta1().HelmReleases(fhr.Namespace), fhr, chartRevision); err != nil {
			chs.logger.Log("warning", "could not update the release revision", "resource", fhr.ResourceID().String(), "err", err)
		}
		return
	}

	if !chs.release.OwnedByHelmRelease(rel, fhr) {
		msg := fmt.Sprintf("release '%s' does not belong to HelmRelease", releaseName)
		chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonUpgradeFailed, msg)
		chs.logger.Log("warning", msg + ", this may be an indication that multiple HelmReleases with the same release name exist", "resource", fhr.ResourceID().String())
		return
	}

	changed, err := chs.shouldUpgrade(chartPath, rel, fhr)
	if err != nil {
		chs.logger.Log("warning", "unable to determine if release has changed", "resource", fhr.ResourceID().String(), "err", err)
		return
	}
	if changed {
		cFhr, err := chs.ifClient.FluxV1beta1().HelmReleases(fhr.Namespace).Get(fhr.Name, metav1.GetOptions{})
		if err != nil {
			chs.logger.Log("warning", "failed to retrieve HelmRelease scheduled for upgrade", "resource", fhr.ResourceID().String(), "err", err)
			return
		}
		if diff := cmp.Diff(fhr.Spec, cFhr.Spec); diff != "" {
			chs.logger.Log("warning", "HelmRelease spec has diverged since we calculated if we should upgrade, skipping upgrade", "resource", fhr.ResourceID().String())
			return
		}
		_, err = chs.release.Install(chartPath, releaseName, fhr, release.UpgradeAction, opts, &chs.kubeClient)
		if err != nil {
			chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionFalse, ReasonUpgradeFailed, err.Error())
			chs.logger.Log("warning", "failed to upgrade chart", "resource", fhr.ResourceID().String(), "err", err)
			return
		}
		chs.setCondition(fhr, fluxv1beta1.HelmReleaseReleased, v1.ConditionTrue, ReasonSuccess, "helm upgrade succeeded")
		if err = status.UpdateReleaseRevision(chs.ifClient.FluxV1beta1().HelmReleases(fhr.Namespace), fhr, chartRevision); err != nil {
			chs.logger.Log("warning", "could not update the release revision", "resource", fhr.ResourceID().String(), "err", err)
		}
		return
	}
}

// DeleteRelease deletes the helm release associated with a
// HelmRelease. This exists mainly so that the operator code can
// call it when it is handling a resource deletion.
func (chs *ChartChangeSync) DeleteRelease(fhr fluxv1beta1.HelmRelease) {
	// FIXME(michael): these may need to stop mirroring a repo.
	name := release.GetReleaseName(fhr)
	err := chs.release.Delete(name)
	if err != nil {
		chs.logger.Log("warning", "chart release not deleted", "resource", fhr.ResourceID().String(), "release", name, "err", err)
	}

	// Remove the clone we may have for this HelmRelease
	chs.clonesMu.Lock()
	cloneForChart, ok := chs.clones[name]
	if ok {
		if cloneForChart.export != nil {
			cloneForChart.export.Clean()
		}
		delete(chs.clones, name)
	}
	chs.clonesMu.Unlock()
}

// SyncMirrors instructs all mirrors to refresh from their upstream.
func (chs *ChartChangeSync) SyncMirrors() {
	chs.logger.Log("info", "starting mirror sync")
	for _, err := range chs.mirrors.RefreshAll(chs.config.GitTimeout) {
		chs.logger.Log("error", fmt.Sprintf("failure while syncing mirror: %s", err))
	}
	chs.logger.Log("info", "finished syncing mirrors")
}

// getCustomResourcesForMirror retrieves all the resources that make
// use of the given mirror from the lister.
func (chs *ChartChangeSync) getCustomResourcesForMirror(mirror string) ([]fluxv1beta1.HelmRelease, error) {
	var fhrs []v1beta1.HelmRelease
	list, err := chs.fhrLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, fhr := range list {
		if fhr.Spec.GitChartSource == nil {
			continue
		}
		if mirror != mirrorName(fhr.Spec.GitChartSource) {
			continue
		}
		fhrs = append(fhrs, *fhr)
	}
	return fhrs, nil
}

// setCondition saves the status of a condition, if it's new
// information. New information is something that adds or changes the
// status, reason or message (i.e., anything but the transition time)
// for one of the types of condition.
func (chs *ChartChangeSync) setCondition(fhr fluxv1beta1.HelmRelease, typ fluxv1beta1.HelmReleaseConditionType, st v1.ConditionStatus, reason, message string) error {
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
		return false, fmt.Errorf("no chart release provided for %v", fhr.GetName())
	}

	currVals := currRel.GetConfig()
	currChart := currRel.GetChart()

	// Get the desired release state
	opts := release.InstallOptions{DryRun: true}
	tempRelName := string(fhr.UID)
	desRel, err := chs.release.Install(chartsRepo, tempRelName, fhr, release.InstallAction, opts, &chs.kubeClient)
	if err != nil {
		return false, err
	}
	desVals := desRel.GetConfig()
	desChart := desRel.GetChart()

	// compare values && Chart
	if diff := cmp.Diff(currVals, desVals); diff != "" {
		if chs.config.LogDiffs {
			chs.logger.Log("error", fmt.Sprintf("release %s: values have diverged due to manual chart release", currRel.GetName()), "resource", fhr.ResourceID().String(), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("release %s: values have diverged due to manual chart release", currRel.GetName()), "resource", fhr.ResourceID().String())
		}
		return true, nil
	}

	if diff := cmp.Diff(sortChartFields(currChart), sortChartFields(desChart)); diff != "" {
		if chs.config.LogDiffs {
			chs.logger.Log("error", fmt.Sprintf("release %s: chart has diverged due to manual chart release", currRel.GetName()), "resource", fhr.ResourceID().String(), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("release %s: chart has diverged due to manual chart release", currRel.GetName()), "resource", fhr.ResourceID().String())
		}
		return true, nil
	}

	return false, nil
}
