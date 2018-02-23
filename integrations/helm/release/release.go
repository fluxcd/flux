package release

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	//	"k8s.io/client-go/kubernetes"

	yaml "gopkg.in/yaml.v2"
	k8shelm "k8s.io/helm/pkg/helm"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/go-kit/kit/log"
	ifv1 "github.com/weaveworks/flux/apis/integrations.flux/v1"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
)

var (
	ErrChartGitPathMissing = "Chart deploy configuration (%s) has empty Chart git path"
)

// ReleaseType determines whether we are making a new Chart release or updating an existing one
type ReleaseType string

// Release contains clients needed to provide functionality related to helm releases
type Release struct {
	logger     log.Logger
	HelmClient *k8shelm.Client
	Repo       repo
	sync.RWMutex
}

type repo struct {
	charts *helmgit.Checkout
}

// New creates a new Release instance
func New(logger log.Logger, helmClient *k8shelm.Client, chartsCheckout *helmgit.Checkout) *Release {
	repo := repo{
		charts: chartsCheckout,
	}
	r := &Release{
		logger:     log.With(logger, "component", "release"),
		HelmClient: helmClient,
		Repo:       repo,
	}

	return r
}

// GetReleaseName either retrieves the release name from the Custom Resource or constructs a new one
//  in the form : $Namespace-$CustomResourceName
func GetReleaseName(fhr ifv1.FluxHelmResource) string {
	namespace := fhr.Namespace
	if namespace == "" {
		namespace = "default"
	}
	releaseName := fhr.Spec.ReleaseName
	fmt.Printf("---> release name from fhr.Spec.ReleaseName: %s\n", releaseName)
	if releaseName == "" {
		releaseName = fmt.Sprintf("%s-%s", namespace, fhr.Name)
	}
	fmt.Printf("---> final release name: %s\n", releaseName)

	return releaseName
}

// Exists ... detects if a particular Chart release exists
// 		release name must match regex ^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$
// Get ... detects if a particular Chart release exists
// 		release name must match regex ^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])+$
func (r *Release) Exists(name string) (bool, error) {
	rls, err := r.HelmClient.ReleaseContent(name)
	if err != nil {
		r.logger.Log("info", fmt.Sprintf("Getting release (%s): %v", name, err))
		return false, err
	}
	/*
		"UNKNOWN":          0,
		"DEPLOYED":         1,
		"DELETED":          2,
		"SUPERSEDED":       3,
		"FAILED":           4,
		"DELETING":         5,
		"PENDING_INSTALL":  6,
		"PENDING_UPGRADE":  7,
		"PENDING_ROLLBACK": 8,
	*/
	rst := rls.Release.Info.Status.GetCode()
	r.logger.Log("info", fmt.Sprintf("Release [%s] status: %#v", name, rst.String()))

	if rst == 1 || rst == 4 {
		r.logger.Log("info", fmt.Sprintf("Release [%s] status: %#v", name, rst.String()))
		return true, nil
	}
	return true, fmt.Errorf("Release [%s] exists with status: %s", name, rst.String())
}

func (r *Release) canDelete(name string) (bool, error) {
	rls, err := r.HelmClient.ReleaseStatus(name)
	r.logger.Log("info", fmt.Sprintf("+++  2 release status = %#v", rls))
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Error finding status for release (%s): %v", name, err))
		return false, err
	}
	/*
		"UNKNOWN":          0,
		"DEPLOYED":         1,
		"DELETED":          2,
		"SUPERSEDED":       3,
		"FAILED":           4,
		"DELETING":         5,
		"PENDING_INSTALL":  6,
		"PENDING_UPGRADE":  7,
		"PENDING_ROLLBACK": 8,
	*/
	status := rls.GetInfo().GetStatus()
	r.logger.Log("info", fmt.Sprintf("Release [%s] status: %#v", name, status.Code.String()))
	switch status.Code {
	case 1, 4:
		r.logger.Log("info", fmt.Sprintf("Deleting release (%s)", name))
		return true, nil
	case 2:
		r.logger.Log("info", fmt.Sprintf("Release (%s) already deleted", name))
		return false, nil
	default:
		r.logger.Log("info", fmt.Sprintf("Release (%s) with status %s cannot be deleted", name, status.Code.String()))
		return false, fmt.Errorf("Release (%s) with status %s cannot be deleted", name, status.Code.String())
	}
}

// Install ... performs Chart release. Depending on the release type, this is either a new release,
// or an upgrade of an existing one
func (r *Release) Install(releaseName string, fhr ifv1.FluxHelmResource, releaseType ReleaseType) (hapi_release.Release, error) {
	r.Lock()
	defer r.Unlock()

	r.logger.Log("info", fmt.Sprintf("releaseName= %s, releaseType=%s", releaseName, releaseType))

	chartPath := fhr.Spec.ChartGitPath
	if chartPath == "" {
		r.logger.Log("error", fmt.Sprintf(ErrChartGitPathMissing, fhr.GetName()))
		return hapi_release.Release{}, fmt.Errorf(ErrChartGitPathMissing, fhr.GetName())
	}

	namespace := fhr.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	err := r.Repo.charts.Pull()
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Failure to do git pull: %#v", err))
		return hapi_release.Release{}, err
	}

	chartDir := filepath.Join(r.Repo.charts.Dir, chartPath)

	rawVals, err := collectValues(fhr.Spec.Customizations)
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Problem with supplied customizations for Chart release [%s]: %#v", releaseName, err))
		return hapi_release.Release{}, err
	}

	// INSTALLATION ----------------------------------------------------------------------
	switch releaseType {
	case "CREATE":
		res, err := r.HelmClient.InstallRelease(
			chartDir,
			namespace,
			k8shelm.ValueOverrides(rawVals),
			k8shelm.ReleaseName(releaseName),
			/*
				helm.InstallDryRun(i.dryRun),
				helm.InstallReuseName(i.replace),
				helm.InstallDisableHooks(i.disableHooks),
				helm.InstallTimeout(i.timeout),
				helm.InstallWait(i.wait)
			*/
		)
		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Chart release failed: %s: %#v", releaseName, err))
			return hapi_release.Release{}, err
		}
		return *res.Release, nil
	case "UPDATE":
		res, err := r.HelmClient.UpdateRelease(
			releaseName,
			chartDir,
			k8shelm.UpdateValueOverrides(rawVals),
			/*
				helm.UpgradeDryRun(u.dryRun),
				helm.UpgradeRecreate(u.recreate),
				helm.UpgradeForce(u.force),
				helm.UpgradeDisableHooks(u.disableHooks),
				helm.UpgradeTimeout(u.timeout),
				helm.ResetValues(u.resetValues),
				helm.ReuseValues(u.reuseValues),
				helm.UpgradeWait(u.wait))
			*/

		)
		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Chart upgrade release failed: %s: %#v", releaseName, err))
			return hapi_release.Release{}, err
		}
		return *res.Release, nil
	default:
		r.logger.Log("error", fmt.Sprintf("Valid ReleaseType options: CREATE, UPDATE. Provided: %s", releaseType))
		return hapi_release.Release{}, err
	}
}

// Delete ... deletes Chart release
func (r *Release) Delete(name string) error {
	r.Lock()
	defer r.Unlock()

	ok, err := r.canDelete(name)
	if !ok {
		if err != nil {
			return err
		}
		return nil
	}

	res, err := r.HelmClient.DeleteRelease(name)
	fmt.Sprintf("===> res: %#v\n\n", res)
	if res != nil {
		fmt.Sprintf("===> res.GetRelease: %#v\n\n", res.GetRelease())
	}

	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Release deletion error: %#v", err))
		return err
	}
	r.logger.Log("info", fmt.Sprintf("Release deleted: [%s]", name))
	return nil
}

// GetAll provides Chart releases (stored in tiller ConfigMaps)
func (r *Release) GetAll() ([]*hapi_release.Release, error) {
	response, err := r.HelmClient.ListReleases()
	if err != nil {
		return nil, r.logger.Log("error", err)
	}
	fmt.Printf("Number of helm releases is %d\n", response.GetCount())

	for i, r := range response.GetReleases() {
		fmt.Printf("\t==> %d : %#v\n\n\t\t\tin namespace %#v\n\n\t\tChartMetadata: %v\n\n\n", i, r.Name, r.Namespace, r.GetChart().GetMetadata())
	}

	return response.GetReleases(), nil
}

func collectValues(params []ifv1.HelmChartParam) ([]byte, error) {
	base := map[string]interface{}{}
	if params == nil || len(params) == 0 {
		return yaml.Marshal(base)
	}

	for _, p := range params {
		k := strings.TrimSpace(p.Name)
		k = strings.Trim(k, "\n")
		if k == "" {
			continue
		}
		v := strings.TrimSpace(p.Value)
		v = strings.Trim(v, "\n")
		base[k] = v
	}

	fmt.Printf("Values string slice ... %#v\n\n", base)
	return yaml.Marshal(base)
}

func (r *Release) tillerCheck(err error) {
	//if _, ok := err.(context.deadlineExceededError; ok {

	//}
}
