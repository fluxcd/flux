/*

This package has the algorithm for making sure the Helm releases in
the cluster match what are defined in the FluxHelmRelease resources.

There are several ways they can be mismatched. Here's how they are
reconciled:

 1a. There is a FluxHelmRelease resource, but no corresponding
   release. This can happen when the helm operator is first run, for
   example. The ChartChangeSync periodically checks for this by
   running through the resources and installing any that aren't
   released already.

 1b. The release corresponding to a FluxHelmRelease has been updated by
   some other means, perhaps while the operator wasn't running. This
   is also checked periodically, by doing a dry-run release and
   comparing the result to the release.

 2. The chart has changed in git, meaning the release is out of
   date. The ChartChangeSync responds to new git commits by looking at
   each chart that's referenced by a FluxHelmRelease, and if it's
   changed since the last seen commit, updating the release.

1a.) and 1b.) run on the same schedule, and 2.) is run when the git
mirror reports it has fetched from upstream _and_ (upon checking) the
head of the branch has changed.

Since both 1*.) and 2.) look at the charts in the git repo, but run on
different schedules (non-deterministically), there's a chance that
they can fight each other. For example, the git mirror may fetch new
commits which are used in 1), then treated as changes subsequently by
2). To keep consistency between the two, the current revision is used
by 1), and advanced only by 2).

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/weaveworks/flux/git"
	fhr "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	helmop "github.com/weaveworks/flux/integrations/helm"
	"github.com/weaveworks/flux/integrations/helm/release"
)

type Polling struct {
	Interval time.Duration
}

type Clients struct {
	KubeClient kubernetes.Clientset
	IfClient   ifclientset.Clientset
}

type ChartChangeSync struct {
	logger log.Logger
	Polling
	kubeClient kubernetes.Clientset
	ifClient   ifclientset.Clientset
	release    *release.Release
	config     helmop.RepoConfig
	logDiffs   bool

	mu    sync.RWMutex
	clone *git.Export
}

func New(logger log.Logger, polling Polling, clients Clients, release *release.Release, config helmop.RepoConfig, logReleaseDiffs bool) *ChartChangeSync {
	return &ChartChangeSync{
		logger:     logger,
		Polling:    polling,
		kubeClient: clients.KubeClient,
		ifClient:   clients.IfClient,
		release:    release,
		config:     config,
		logDiffs:   logReleaseDiffs,
	}
}

// Run creates a syncing loop monitoring repo chart changes. It is
// assumed that the *git.Repo given to the config is ready to use
// before this is invoked.
//
// The behaviour if the git mirror becomes unavailable while it's
// running is not defined (this could be tightened up).
func (chs *ChartChangeSync) Run(stopCh <-chan struct{}, errc chan error, wg *sync.WaitGroup) {
	chs.logger.Log("info", "Starting charts sync loop")
	wg.Add(1)
	go func() {
		defer runtime.HandleCrash()
		defer wg.Done()

		ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
		currentRevision, err := chs.config.Repo.Revision(ctx, chs.config.Branch)
		if err == nil {
			chs.mu.Lock()
			chs.clone, err = chs.config.Repo.Export(ctx, currentRevision)
			chs.mu.Unlock()
		}
		cancel()
		if err != nil {
			errc <- err
			return
		}
		defer chs.clone.Clean()

		ticker := time.NewTicker(chs.Polling.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-chs.config.Repo.C:
				ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
				head, err := chs.config.Repo.Revision(ctx, chs.config.Branch)
				cancel()
				if err != nil {
					chs.logger.Log("warning", "failure using git repo", "error", err.Error())
					continue
				}

				if head == currentRevision {
					chs.logger.Log("info", "no new commits on branch", "branch", chs.config.Branch, "head", head)
					continue
				}

				ctx, cancel = context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
				newClone, err := chs.config.Repo.Export(ctx, head)
				cancel()
				if err != nil {
					chs.logger.Log("warning", "failure to clone git repo", "error", err)
					continue
				}
				chs.mu.Lock()
				chs.clone.Clean()
				chs.clone = newClone
				chs.mu.Unlock()

				chs.logger.Log("info", fmt.Sprint("Start of chartsync"))
				err = chs.applyChartChanges(currentRevision, head)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do chart sync: %s", err))
				}
				currentRevision = head
				chs.logger.Log("info", fmt.Sprint("End of chartsync"))

			case <-ticker.C:
				// Re-release any chart releases that have apparently
				// changed in the cluster.
				chs.logger.Log("info", fmt.Sprint("Start of releasesync"))
				err = chs.reapplyReleaseDefs()
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

// ReconcileReleaseDef asks the ChartChangeSync to examine the release
// associated with a FluxHelmRelease, compared to what is in the git
// repo, and install or upgrade the release if necessary. This may
// block indefinitely, so the caller provides a context which it can
// cancel if it gets tired of waiting. Returns an error if the context
// timed out or was canceled before the operation was started.
func (chs *ChartChangeSync) ReconcileReleaseDef(fhr fhr.FluxHelmRelease) {
	chs.reconcileReleaseDef(fhr)
}

// ApplyChartChanges looks at the FluxHelmRelease resources in the
// cluster, figures out which refer to charts that have changed since
// the last commit, then re-releases those that have.
func (chs *ChartChangeSync) applyChartChanges(prevRef, head string) error {
	resources, err := chs.getCustomResources()
	if err != nil {
		return fmt.Errorf("Failure getting FHR custom resources: %s", err.Error())
	}

	// Release all the resources whose charts have changed. More than
	// one FluxHelmRelease resource can refer to a given chart, so to
	// avoid repeated checking, keep track of which charts have
	// changed or not changed.
	chartHasChanged := map[string]bool{}

	for _, fhr := range resources {
		if fhr.Spec.ChartSource.GitChartSource == nil {
			continue
		}
		chartPath := filepath.Join(chs.config.ChartsPath, fhr.Spec.ChartSource.GitChartSource.Path)
		changed, ok := chartHasChanged[chartPath]
		if !ok {
			ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
			commits, err := chs.config.Repo.CommitsBetween(ctx, prevRef, head, chartPath)
			cancel()
			if err != nil {
				return fmt.Errorf("error while checking if chart at %q has changed in %s..%s: %s", chartPath, prevRef, head, err.Error())
			}
			changed = len(commits) > 0
			chartHasChanged[chartPath] = changed
		}
		if changed {
			rlsName := release.GetReleaseName(fhr)
			opts := release.InstallOptions{DryRun: false}
			chs.mu.RLock()
			if _, err = chs.release.Install(chs.clone.Dir(), rlsName, fhr, release.UpgradeAction, opts, &chs.kubeClient); err != nil {
				// NB in this step, failure to release is considered non-fatal, i.e,. we move on to the next rather than giving up entirely.
				chs.logger.Log("warning", "failure to release chart with changes in git", "error", err, "chart", chartPath, "release", rlsName)
			}
			chs.mu.RUnlock()
		}
	}

	return nil
}

// reconcileReleaseDef looks up the helm release associated with a
// FluxHelmRelease resource, and either installs, upgrades, or does
// nothing, depending on the state (or absence) of the release.
func (chs *ChartChangeSync) reconcileReleaseDef(fhr fhr.FluxHelmRelease) {
	releaseName := release.GetReleaseName(fhr)

	// There's no exact way in the Helm API to test whether a release
	// exists or not. Instead, try to fetch it, and treat an error as
	// not existing (and possibly fail further below, if it meant
	// something else).
	rel, _ := chs.release.GetDeployedRelease(releaseName)

	chs.mu.RLock()
	defer chs.mu.RUnlock()

	opts := release.InstallOptions{DryRun: false}
	if rel == nil {
		_, err := chs.release.Install(chs.clone.Dir(), releaseName, fhr, release.InstallAction, opts, &chs.kubeClient)
		if err != nil {
			chs.logger.Log("warning", "Failed to install chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
		}
		return
	}

	changed, err := chs.shouldUpgrade(chs.clone.Dir(), rel, fhr)
	if err != nil {
		chs.logger.Log("warning", "Unable to determine if release has changed", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
		return
	}
	if changed {
		_, err := chs.release.Install(chs.clone.Dir(), releaseName, fhr, release.UpgradeAction, opts, &chs.kubeClient)
		if err != nil {
			chs.logger.Log("warning", "Failed to upgrade chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
		}
	}
}

// reapplyReleaseDefs goes through the resource definitions and
// reconciles them with Helm releases. This is a "backstop" for the
// other sync processes, to cover the case of a release being changed
// out-of-band (e.g., by someone using `helm upgrade`).
func (chs *ChartChangeSync) reapplyReleaseDefs() error {
	resources, err := chs.getCustomResources()
	if err != nil {
		return fmt.Errorf("failed to get FluxHelmRelease resources from the API server: %s", err.Error())
	}

	for _, fhr := range resources {
		chs.reconcileReleaseDef(fhr)
	}
	return nil
}

// DeleteRelease deletes the helm release associated with a
// FluxHelmRelease. This exists mainly so that the operator code can
// call it when it is handling a resource deletion.
func (chs *ChartChangeSync) DeleteRelease(fhr fhr.FluxHelmRelease) {
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
func (chs *ChartChangeSync) getCustomResources() ([]fhr.FluxHelmRelease, error) {
	namespaces, err := chs.getNamespaces()
	if err != nil {
		return nil, err
	}

	var fhrs []fhr.FluxHelmRelease
	for _, ns := range namespaces {
		list, err := chs.ifClient.FluxV1beta1().FluxHelmReleases(ns).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, fhr := range list.Items {
			fhrs = append(fhrs, fhr)
		}
	}
	return fhrs, nil
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
func (chs *ChartChangeSync) shouldUpgrade(chartsRepo string, currRel *hapi_release.Release, fhr fhr.FluxHelmRelease) (bool, error) {
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
		if chs.logDiffs {
			chs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()))
		}
		return true, nil
	}

	if diff := cmp.Diff(sortChartFields(currChart), sortChartFields(desChart)); diff != "" {
		if chs.logDiffs {
			chs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()), "diff", diff)
		} else {
			chs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()))
		}
		return true, nil
	}

	return false, nil
}
