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
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	"github.com/weaveworks/flux/git"
	ifclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	helmop "github.com/weaveworks/flux/integrations/helm"
	"github.com/weaveworks/flux/integrations/helm/release"
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
	kubeClient kubernetes.Clientset
	ifClient   ifclientset.Clientset
	release    *release.Release
	config     helmop.RepoConfig
}

func New(logger log.Logger, polling Polling, clients Clients, release *release.Release, config helmop.RepoConfig) *ChartChangeSync {
	return &ChartChangeSync{
		logger:     logger,
		Polling:    polling,
		kubeClient: clients.KubeClient,
		ifClient:   clients.IfClient,
		release:    release,
		config:     config,
	}
}

//  Run creates a syncing loop monitoring repo chart changes. It is
//  assumed that the *git.Repo given to the config is ready to use
//  before this is invoked.
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
		cancel()
		if err != nil {
			errc <- err
			return
		}

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

				// Sync changes to charts in the git repo
				chs.logger.Log("info", fmt.Sprint("Start of chartsync"))
				err = chs.ApplyChartChanges(currentRevision, head)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do chart sync: %#v", err))
				}
				currentRevision = head
				chs.logger.Log("info", fmt.Sprint("End of chartsync"))

			case <-ticker.C:
				// Re-release any chart releases that have apparently
				// changed in the cluster.
				chs.logger.Log("info", fmt.Sprint("Start of releasesync"))
				err = chs.ReapplyReleaseDefs(currentRevision)
				if err != nil {
					chs.logger.Log("error", fmt.Sprintf("Failure to do manual release sync: %s", err))
				}
				chs.logger.Log("info", fmt.Sprint("End of releasesync"))

			case <-stopCh:
				chs.logger.Log("stopping", "true")
				break
			}
		}
	}()
}

// ApplyChartChanges looks at the FluxHelmRelease resources in the
// cluster, figures out which refer to charts that have changed since
// the last commit, then re-releases those that have.
func (chs *ChartChangeSync) ApplyChartChanges(prevRef, head string) error {
	resources, err := chs.getCustomResources()
	if err != nil {
		return fmt.Errorf("Failure getting FHR custom resources: %s", err.Error())
	}

	// Release all the resources whose charts have changed. More than
	// one FluxHelmRelease resource can refer to a given chart, so to
	// avoid repeated checking, keep track of which charts have
	// changed or not changed.
	chartHasChanged := map[string]bool{}

	// Lazily clone the repo if and when it turns out we need it
	var clone *git.Export
	defer func() {
		if clone != nil {
			clone.Clean()
		}
	}()

	for _, fhr := range resources {
		chartPath := filepath.Join(chs.config.ChartsPath, fhr.Spec.ChartGitPath)
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
			if clone == nil {
				clone, err = chs.exportAtRef(head)
				if err != nil {
					return fmt.Errorf("failed to clone repo at %s: %s", head, err.Error())
				}
			}

			rlsName := release.GetReleaseName(fhr)
			opts := release.InstallOptions{DryRun: false}
			if _, err = chs.release.Install(clone.Dir(), rlsName, fhr, release.UpgradeAction, opts); err != nil {
				// NB in this step, failure to release is considered non-fatal, i.e,. we move on to the next rather than giving up entirely.
				chs.logger.Log("warning", "failure to release chart with changes in git", "error", err, "chart", chartPath, "release", rlsName)
			}
		}
	}

	return nil
}

func (chs *ChartChangeSync) ReapplyReleaseDefs(ref string) error {
	var clone *git.Export
	defer func() {
		if clone != nil {
			clone.Clean()
		}
	}()

	resources, err := chs.getCustomResources()
	if err != nil {
		return fmt.Errorf("failed to get FluxHelmRelease resources from the API server: %s", err.Error())
	}

	for _, fhr := range resources {
		releaseName := release.GetReleaseName(fhr)
		rel, err := chs.release.GetDeployedRelease(releaseName)
		if err != nil {
			return fmt.Errorf("failed to get release %q: %s", releaseName, err)
		}

		// At this point, one way or another, we are going to need a clone of the repo.
		if clone == nil {
			clone, err = chs.exportAtRef(ref)
			if err != nil {
				return err
			}
		}

		opts := release.InstallOptions{DryRun: false}
		if rel == nil {
			_, err := chs.release.Install(clone.Dir(), releaseName, fhr, release.InstallAction, opts)
			if err != nil {
				chs.logger.Log("warning", "Failed to install chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
			}
			continue
		}

		changed, err := chs.shouldUpgrade(clone.Dir(), rel, fhr)
		if err != nil {
			chs.logger.Log("warning", "Unable to determine if release has changed", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
			continue
		}
		if changed {
			_, err := chs.release.Install(clone.Dir(), releaseName, fhr, release.UpgradeAction, opts)
			if err != nil {
				chs.logger.Log("warning", "Failed to upgrade chart", "namespace", fhr.Namespace, "name", fhr.Name, "error", err)
			}
		}
	}
	return nil
}

//---

func (chs *ChartChangeSync) exportAtRef(ref string) (*git.Export, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helmop.GitOperationTimeout)
	clone, err := chs.config.Repo.Export(ctx, ref)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("error cloning repo at ref %s for chart releases: %s", ref, err.Error())
	}
	return clone, nil
}

// GetNamespaces gets current kubernetes cluster namespaces
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

// getCustomResources assembles all custom resources
func (chs *ChartChangeSync) getCustomResources() ([]ifv1.FluxHelmRelease, error) {
	namespaces, err := chs.getNamespaces()
	if err != nil {
		return nil, err
	}

	var fhrs []ifv1.FluxHelmRelease
	for _, ns := range namespaces {
		list, err := chs.ifClient.HelmV1alpha2().FluxHelmReleases(ns).List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		for _, fhr := range list.Items {
			fhrs = append(fhrs, fhr)
		}
	}
	return fhrs, nil
}

// shouldUpgrade returns true if the current running values or chart
// don't match what the repo says we ought to be running, based on
// doing a dry run install from the chart in the git repo.
func (chs *ChartChangeSync) shouldUpgrade(chartsRepo string, currRel *hapi_release.Release, fhr ifv1.FluxHelmRelease) (bool, error) {
	if currRel == nil {
		return false, fmt.Errorf("No Chart release provided for %v", fhr.GetName())
	}

	currVals := currRel.GetConfig().GetRaw()
	currChart := currRel.GetChart().String()

	// Get the desired release state
	opts := release.InstallOptions{DryRun: true}
	tempRelName := currRel.GetName() + "-temp"
	desRel, err := chs.release.Install(chartsRepo, tempRelName, fhr, release.InstallAction, opts)
	if err != nil {
		return false, err
	}
	desVals := desRel.GetConfig().GetRaw()
	desChart := desRel.GetChart().String()

	// compare values && Chart
	if currVals != desVals {
		chs.logger.Log("error", fmt.Sprintf("Release %s: values have diverged due to manual Chart release", currRel.GetName()))
		return true, nil
	}
	if currChart != desChart {
		chs.logger.Log("error", fmt.Sprintf("Release %s: Chart has diverged due to manual Chart release", currRel.GetName()))
		return true, nil
	}

	return false, nil
}
