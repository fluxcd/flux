package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/chartutil"
	k8shelm "k8s.io/helm/pkg/helm"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/weaveworks/flux"
	fluxk8s "github.com/weaveworks/flux/cluster/kubernetes"
	flux_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
)

type Action string

const (
	InstallAction Action = "CREATE"
	UpgradeAction Action = "UPDATE"
)

// Release contains clients needed to provide functionality related to helm releases
type Release struct {
	logger log.Logger

	HelmClient *k8shelm.Client
}

type Releaser interface {
	GetCurrent() (map[string][]DeployInfo, error)
	GetDeployedRelease(name string) (*hapi_release.Release, error)
	Install(dir string, releaseName string, fhr flux_v1beta1.FluxHelmRelease, action Action, opts InstallOptions) (*hapi_release.Release, error)
}

type DeployInfo struct {
	Name string
}

type InstallOptions struct {
	DryRun    bool
	ReuseName bool
}

// New creates a new Release instance.
func New(logger log.Logger, helmClient *k8shelm.Client) *Release {
	r := &Release{
		logger:     logger,
		HelmClient: helmClient,
	}
	return r
}

// GetReleaseName either retrieves the release name from the Custom Resource or constructs a new one
//  in the form : $Namespace-$CustomResourceName
func GetReleaseName(fhr flux_v1beta1.FluxHelmRelease) string {
	namespace := fhr.Namespace
	if namespace == "" {
		namespace = "default"
	}
	releaseName := fhr.Spec.ReleaseName
	if releaseName == "" {
		releaseName = fmt.Sprintf("%s-%s", namespace, fhr.Name)
	}

	return releaseName
}

// GetDeployedRelease returns a release with Deployed status
func (r *Release) GetDeployedRelease(name string) (*hapi_release.Release, error) {
	rls, err := r.HelmClient.ReleaseContent(name)
	if err != nil {
		return nil, err
	}
	if rls.Release.Info.Status.GetCode() == hapi_release.Status_DEPLOYED {
		return rls.GetRelease(), nil
	}
	return nil, nil
}

func (r *Release) canDelete(name string) (bool, error) {
	rls, err := r.HelmClient.ReleaseStatus(name)

	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Error finding status for release (%s): %#v", name, err))
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
	r.logger.Log("info", fmt.Sprintf("Release [%s] status: %s", name, status.Code.String()))
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

// Install performs a Chart release given the directory containing the
// charts, and the FluxHelmRelease specifying the release. Depending
// on the release type, this is either a new release, or an upgrade of
// an existing one.
//
// TODO(michael): cloneDir is only relevant if installing from git;
// either split this procedure into two varieties, or make it more
// general and calculate the path to the chart in the caller.
func (r *Release) Install(chartPath, releaseName string, fhr flux_v1beta1.FluxHelmRelease, action Action, opts InstallOptions, kubeClient *kubernetes.Clientset) (*hapi_release.Release, error) {
	if chartPath == "" {
		return nil, fmt.Errorf("empty path to chart supplied for resource %q", fhr.ResourceID().String())
	}
	_, err := os.Stat(chartPath)
	switch {
	case os.IsNotExist(err):
		return nil, fmt.Errorf("no file or dir at path to chart: %s", chartPath)
	case err != nil:
		return nil, fmt.Errorf("error statting path given for chart %s: %s", chartPath, err.Error())
	}

	r.logger.Log("info", "releaseName", releaseName, "action", action, "options", fmt.Sprintf("%+v", opts))

	// Read values from given valueFile paths (configmaps, etc.)
	mergedValues := chartutil.Values{}
	for _, valueFileSecret := range fhr.Spec.ValueFileSecrets {
		// Read the contents of the secret
		secret, err := kubeClient.CoreV1().Secrets(fhr.Namespace).Get(valueFileSecret.Name, v1.GetOptions{})
		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Cannot get secret %s for Chart release [%s]: %#v", valueFileSecret.Name, releaseName, err))
			return nil, err
		}

		// Load values.yaml file and merge
		var values chartutil.Values
		err = yaml.Unmarshal(secret.Data["values.yaml"], &values)
		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Cannot yaml.Unmashal values.yaml in secret %s for Chart release [%s]: %#v", valueFileSecret.Name, releaseName, err))
			return nil, err
		}
		mergedValues = mergeValues(mergedValues, values)
	}
	// Merge in values after valueFiles
	mergedValues = mergeValues(mergedValues, fhr.Spec.Values)

	strVals, err := mergedValues.YAML()
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Problem with supplied customizations for Chart release [%s]: %#v", releaseName, err))
		return nil, err
	}
	rawVals := []byte(strVals)

	switch action {
	case InstallAction:
		res, err := r.HelmClient.InstallRelease(
			chartPath,
			fhr.GetNamespace(),
			k8shelm.ValueOverrides(rawVals),
			k8shelm.ReleaseName(releaseName),
			k8shelm.InstallDryRun(opts.DryRun),
			k8shelm.InstallReuseName(opts.ReuseName),
			/*
				helm.InstallReuseName(i.replace),
				helm.InstallDisableHooks(i.disableHooks),
				helm.InstallTimeout(i.timeout),
				helm.InstallWait(i.wait)
			*/
		)

		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Chart release failed: %s: %#v", releaseName, err))
			// if an install fails, purge the release and keep retrying
			r.logger.Log("info", fmt.Sprintf("Deleting failed release: [%s]", releaseName))
			_, err = r.HelmClient.DeleteRelease(releaseName, k8shelm.DeletePurge(true))
			if err != nil {
				r.logger.Log("error", fmt.Sprintf("Release deletion error: %#v", err))
				return nil, err
			}
			return nil, err
		}
		if !opts.DryRun {
			err = r.annotateResources(res.Release, fhr)
		}
		return res.Release, err
	case UpgradeAction:
		res, err := r.HelmClient.UpdateRelease(
			releaseName,
			chartPath,
			k8shelm.UpdateValueOverrides(rawVals),
			k8shelm.UpgradeDryRun(opts.DryRun),
			/*
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
			return nil, err
		}
		if !opts.DryRun {
			err = r.annotateResources(res.Release, fhr)
		}
		return res.Release, err
	default:
		err = fmt.Errorf("Valid install options: CREATE, UPDATE. Provided: %s", action)
		r.logger.Log("error", err.Error())
		return nil, err
	}
}

// Delete purges a Chart release
func (r *Release) Delete(name string) error {
	ok, err := r.canDelete(name)
	if !ok {
		if err != nil {
			return err
		}
		return nil
	}

	_, err = r.HelmClient.DeleteRelease(name, k8shelm.DeletePurge(true))
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Release deletion error: %#v", err))
		return err
	}
	r.logger.Log("info", fmt.Sprintf("Release deleted: [%s]", name))
	return nil
}

// GetCurrent provides Chart releases (stored in tiller ConfigMaps)
//		output:
//						map[namespace][release name] = nil
func (r *Release) GetCurrent() (map[string][]DeployInfo, error) {
	response, err := r.HelmClient.ListReleases()
	if err != nil {
		return nil, r.logger.Log("error", err)
	}
	r.logger.Log("info", fmt.Sprintf("Number of Chart releases: %d\n", response.GetCount()))

	relsM := make(map[string][]DeployInfo)
	var depl []DeployInfo

	for _, r := range response.GetReleases() {
		ns := r.Namespace
		depl = relsM[ns]

		depl = append(depl, DeployInfo{Name: r.Name})
		relsM[ns] = depl
	}
	return relsM, nil
}

// annotateResources annotates each of the resources created (or updated)
// by the release so that we can spot them.
func (r *Release) annotateResources(release *hapi_release.Release, fhr flux_v1beta1.FluxHelmRelease) error {
	args := []string{"annotate", "--overwrite"}
	args = append(args, "--namespace", release.Namespace)
	args = append(args, "-f", "-")
	args = append(args, fluxk8s.AntecedentAnnotation+"="+fhrResourceID(fhr).String())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdin = bytes.NewBufferString(release.Manifest)

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.logger.Log("output", string(output), "err", err)
	}
	return err
}

// fhrResourceID constructs a flux.ResourceID for a FluxHelmRelease
// resource.
func fhrResourceID(fhr flux_v1beta1.FluxHelmRelease) flux.ResourceID {
	return flux.MakeResourceID(fhr.Namespace, "FluxHelmRelease", fhr.Name)
}

// Merges source and destination `chartutils.Values`, preferring values from the source Values
// This is slightly adapted from https://github.com/helm/helm/blob/master/cmd/helm/install.go#L329
func mergeValues(dest, src chartutil.Values) chartutil.Values {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}
