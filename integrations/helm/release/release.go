package release

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	k8sclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/getter"
	k8shelm "k8s.io/helm/pkg/helm"
	helmenv "k8s.io/helm/pkg/helm/environment"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/weaveworks/flux"
	fluxk8s "github.com/weaveworks/flux/cluster/kubernetes"
	flux_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	helmutil "k8s.io/helm/pkg/releaseutil"
)

type Action string

const (
	InstallAction Action = "CREATE"
	UpgradeAction Action = "UPDATE"
)

// Release contains clients needed to provide functionality related to helm releases
type Release struct {
	logger     log.Logger
	HelmClient *k8shelm.Client
}

type Releaser interface {
	GetUpgradableRelease(name string) (*hapi_release.Release, error)
	Install(dir string, releaseName string, fhr flux_v1beta1.HelmRelease, action Action, opts InstallOptions) (*hapi_release.Release, error)
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
// in the form : $Namespace-$CustomResourceName
func GetReleaseName(fhr flux_v1beta1.HelmRelease) string {
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

// GetUpgradableRelease returns a release if the current state of it
// allows an upgrade, a descriptive error if it is not allowed, or
// nil if the release does not exist.
func (r *Release) GetUpgradableRelease(name string) (*hapi_release.Release, error) {
	rls, err := r.HelmClient.ReleaseContent(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}

	release := rls.GetRelease()
	status := release.GetInfo().GetStatus()

	switch status.GetCode() {
	case hapi_release.Status_DEPLOYED:
		return release, nil
	case hapi_release.Status_FAILED:
		return nil, fmt.Errorf("release requires a rollback before it can be upgraded (%s)", status.GetCode().String())
	case hapi_release.Status_PENDING_INSTALL,
	     hapi_release.Status_PENDING_UPGRADE,
	     hapi_release.Status_PENDING_ROLLBACK:
		return nil, fmt.Errorf("operation pending for release (%s)", status.GetCode().String())
	default:
		return nil, fmt.Errorf("current state prevents it from being upgraded (%s)", status.GetCode().String())
	}
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
	switch status.Code {
	case 1, 4:
		r.logger.Log("info", fmt.Sprintf("Deleting release %s", name))
		return true, nil
	case 2:
		r.logger.Log("info", fmt.Sprintf("Release %s already deleted", name))
		return false, nil
	default:
		r.logger.Log("info", fmt.Sprintf("Release %s with status %s cannot be deleted", name, status.Code.String()))
		return false, fmt.Errorf("release %s with status %s cannot be deleted", name, status.Code.String())
	}
}

// Install performs a Chart release given the directory containing the
// charts, and the HelmRelease specifying the release. Depending
// on the release type, this is either a new release, or an upgrade of
// an existing one.
//
// TODO(michael): cloneDir is only relevant if installing from git;
// either split this procedure into two varieties, or make it more
// general and calculate the path to the chart in the caller.
func (r *Release) Install(chartPath, releaseName string, fhr flux_v1beta1.HelmRelease, action Action, opts InstallOptions, kubeClient *kubernetes.Clientset) (*hapi_release.Release, error) {
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

	r.logger.Log("info", fmt.Sprintf("processing release %s (as %s)", GetReleaseName(fhr), releaseName),
		"action", fmt.Sprintf("%v", action),
		"options", fmt.Sprintf("%+v", opts),
		"timeout", fmt.Sprintf("%vs", fhr.GetTimeout()))

	valuesFrom := fhr.Spec.ValuesFrom
	// Maintain backwards compatibility with ValueFileSecrets
	if fhr.Spec.ValueFileSecrets != nil {
		var secretKeyRefs []flux_v1beta1.ValuesFromSource
		for _, ref := range fhr.Spec.ValueFileSecrets {
			s := &v1.SecretKeySelector{LocalObjectReference: ref}
			secretKeyRefs = append(secretKeyRefs, flux_v1beta1.ValuesFromSource{SecretKeyRef: s})
		}
		valuesFrom = append(secretKeyRefs, valuesFrom...)
	}
	vals, err := values(kubeClient.CoreV1(), fhr.Namespace, chartPath, valuesFrom, fhr.Spec.Values)
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Failed to compose values for Chart release [%s]: %v", fhr.Spec.ReleaseName, err))
		return nil, err
	}

	strVals, err := vals.YAML()
	if err != nil {
		r.logger.Log("error", fmt.Sprintf("Problem with supplied customizations for Chart release [%s]: %v", fhr.Spec.ReleaseName, err))
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
			k8shelm.InstallTimeout(fhr.GetTimeout()),
			k8shelm.InstallDescription(fhrResourceID(fhr).String()),
		)

		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Chart release failed: %s: %#v", fhr.Spec.ReleaseName, err))
			// purge the release if the install failed but only if this is the first revision
			history, err := r.HelmClient.ReleaseHistory(releaseName, k8shelm.WithMaxHistory(2))
			if err == nil && len(history.Releases) == 1 && history.Releases[0].Info.Status.Code == hapi_release.Status_FAILED {
				r.logger.Log("info", fmt.Sprintf("Deleting failed release: [%s]", fhr.Spec.ReleaseName))
				_, err = r.HelmClient.DeleteRelease(releaseName, k8shelm.DeletePurge(true))
				if err != nil {
					r.logger.Log("error", fmt.Sprintf("Release deletion error: %#v", err))
					return nil, err
				}
			}
			return nil, err
		}
		if !opts.DryRun {
			r.annotateResources(res.Release, fhr)
		}
		return res.Release, err
	case UpgradeAction:
		res, err := r.HelmClient.UpdateRelease(
			releaseName,
			chartPath,
			k8shelm.UpdateValueOverrides(rawVals),
			k8shelm.UpgradeDryRun(opts.DryRun),
			k8shelm.UpgradeTimeout(fhr.GetTimeout()),
			k8shelm.ResetValues(fhr.Spec.ResetValues),
			k8shelm.UpgradeForce(fhr.Spec.ForceUpgrade),
			k8shelm.UpgradeDescription(fhrResourceID(fhr).String()),
		)

		if err != nil {
			r.logger.Log("error", fmt.Sprintf("Chart upgrade release failed: %s: %#v", fhr.Spec.ReleaseName, err))
			return nil, err
		}
		if !opts.DryRun {
			r.annotateResources(res.Release, fhr)
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

// OwnedByHelmRelease validates the release is managed by the given
// HelmRelease, by looking for the resource ID in the release
// description. This validation is necessary because we can not
// validate the uniqueness of a release name on the creation of a
// HelmRelease, which would result in the operator attempting to
// upgrade a release indefinitely when multiple HelmReleases with the
// same release name exist.
//
// For backwards compatibility, and to be able to migrate existing
// releases to a HelmRelease, we define empty descriptions as a
// positive.
func (r *Release) OwnedByHelmRelease(release *hapi_release.Release, fhr flux_v1beta1.HelmRelease) bool {
	description := release.Info.Description

	return description == "" || description == fhrResourceID(fhr).String()
}

// annotateResources annotates each of the resources created (or updated)
// by the release so that we can spot them.
func (r *Release) annotateResources(release *hapi_release.Release, fhr flux_v1beta1.HelmRelease) {
	objs := releaseManifestToUnstructured(release.Manifest, r.logger)
	for namespace, res := range namespacedResourceMap(objs, release.Namespace) {
		args := []string{"annotate", "--overwrite"}
		args = append(args, "--namespace", namespace)
		args = append(args, res...)
		args = append(args, fluxk8s.AntecedentAnnotation+"="+fhrResourceID(fhr).String())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "kubectl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			r.logger.Log("output", string(output), "err", err)
		}
	}
}

// fhrResourceID constructs a flux.ResourceID for a HelmRelease resource.
func fhrResourceID(fhr flux_v1beta1.HelmRelease) flux.ResourceID {
	return flux.MakeResourceID(fhr.Namespace, "HelmRelease", fhr.Name)
}

// values tries to resolve all given value file sources and merges
// them into one Values struct. It returns the merged Values.
func values(corev1 k8sclientv1.CoreV1Interface, ns string, chartPath string, valuesFromSource []flux_v1beta1.ValuesFromSource, values chartutil.Values) (chartutil.Values, error) {
	result := chartutil.Values{}

	for _, v := range valuesFromSource {
		var valueFile chartutil.Values

		switch {
		case v.ConfigMapKeyRef != nil:
			cm := v.ConfigMapKeyRef
			name := cm.Name
			key := cm.Key
			if key == "" {
				key = "values.yaml"
			}
			optional := cm.Optional != nil && *cm.Optional
			configMap, err := corev1.ConfigMaps(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return result, err
			}
			d, ok := configMap.Data[key]
			if !ok {
				if optional {
					continue
				}
				return result, fmt.Errorf("could not find key %v in ConfigMap %s/%s", key, ns, name)
			}
			if err := yaml.Unmarshal([]byte(d), &valueFile); err != nil {
				if optional {
					continue
				}
				return result, fmt.Errorf("unable to yaml.Unmarshal %v from %s in ConfigMap %s/%s", d, key, ns, name)
			}
		case v.SecretKeyRef != nil:
			s := v.SecretKeyRef
			name := s.Name
			key := s.Key
			if key == "" {
				key = "values.yaml"
			}
			optional := s.Optional != nil && *s.Optional
			secret, err := corev1.Secrets(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return result, err
			}
			d, ok := secret.Data[key]
			if !ok {
				if optional {
					continue
				}
				return result, fmt.Errorf("could not find key %s in Secret %s/%s", key, ns, name)
			}
			if err := yaml.Unmarshal(d, &valueFile); err != nil {
				return result, fmt.Errorf("unable to yaml.Unmarshal %v from %s in Secret %s/%s", d, key, ns, name)
			}
		case v.ExternalSourceRef != nil:
			es := v.ExternalSourceRef
			url := es.URL
			optional := es.Optional != nil && *es.Optional
			b, err := readURL(url)
			if err != nil {
				if optional {
					continue
				}
				return result, fmt.Errorf("unable to read value file from URL %s", url)
			}
			if err := yaml.Unmarshal(b, &valueFile); err != nil {
				if optional {
					continue
				}
				return result, fmt.Errorf("unable to yaml.Unmarshal %v from URL %s", b, url)
			}
		case v.ChartFileRef != nil:
			cf := v.ChartFileRef
			filePath := cf.Path
			optional := cf.Optional != nil && *cf.Optional
			f, err := readLocalChartFile(filepath.Join(chartPath, filePath))
			if err != nil {
				if optional {
					continue
				}
				return result, fmt.Errorf("unable to read value file from path %s", filePath)
			}
			if err := yaml.Unmarshal(f, &valueFile); err != nil {
				if optional {
					continue
				}
				return result, fmt.Errorf("unable to yaml.Unmarshal %v from URL %s", f, filePath)
			}
		}

		result = mergeValues(result, valueFile)
	}

	result = mergeValues(result, values)

	return result, nil
}

// Merges source and destination `chartutils.Values`, preferring values from the source Values
// This is slightly adapted from https://github.com/helm/helm/blob/2332b480c9cb70a0d8a85247992d6155fbe82416/cmd/helm/install.go#L359
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

// readURL attempts to read a file from an url.
// This is slightly adapted from https://github.com/helm/helm/blob/2332b480c9cb70a0d8a85247992d6155fbe82416/cmd/helm/install.go#L552
func readURL(URL string) ([]byte, error) {
	var settings helmenv.EnvSettings
	flags := pflag.NewFlagSet("helm-env", pflag.ContinueOnError)
	settings.AddFlags(flags)
	settings.Init(flags)

	u, _ := url.Parse(URL)
	p := getter.All(settings)

	getterConstructor, err := p.ByScheme(u.Scheme)

	if err != nil {
		return []byte{}, err
	}

	getter, err := getterConstructor(URL, "", "", "")
	if err != nil {
		return []byte{}, err
	}
	data, err := getter.Get(URL)
	return data.Bytes(), err
}

// readLocalChartFile attempts to read a file from the chart path.
func readLocalChartFile(filePath string) ([]byte, error) {
	f, err := ioutil.ReadFile(filePath)
	if err != nil {
		return []byte{}, err
	}

	return f, nil
}

// releaseManifestToUnstructured turns a string containing YAML
// manifests into an array of Unstructured objects.
func releaseManifestToUnstructured(manifest string, logger log.Logger) []unstructured.Unstructured {
	manifests := helmutil.SplitManifests(manifest)
	var objs []unstructured.Unstructured
	for _, manifest := range manifests {
		bytes, err := yaml.YAMLToJSON([]byte(manifest))
		if err != nil {
			logger.Log("err", err)
			continue
		}

		var u unstructured.Unstructured
		if err := u.UnmarshalJSON(bytes); err != nil {
			logger.Log("err", err)
			continue
		}

		// Helm charts may include list kinds, we are only interested in
		// the items on those lists.
		if u.IsList() {
			l, err := u.ToList()
			if err != nil {
				logger.Log("err", err)
				continue
			}
			objs = append(objs, l.Items...)
			continue
		}

		objs = append(objs, u)
	}
	return objs
}

// namespacedResourceMap iterates over the given objects and maps the
// resource identifier against the namespace from the object, if no
// namespace is present (either because the object kind has no namespace
// or it belongs to the release namespace) it gets mapped against the
// given release namespace.
func namespacedResourceMap(objs []unstructured.Unstructured, releaseNamespace string) map[string][]string {
	resources := make(map[string][]string)
	for _, obj := range objs {
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = releaseNamespace
		}
		resource := obj.GetKind() + "/" + obj.GetName()
		resources[namespace] = append(resources[namespace], resource)
	}
	return resources
}
