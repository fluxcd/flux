/*

This package is for maintaining the link between `FluxHelmRelease`
resources and the Helm releases to which they
correspond. Specifically,

 1. updating the `FluxHelmRelease` status based on the progress of
   syncing, and the state of the associated Helm release; and,

 2. attributing each resource in a Helm release (under our control) to
 the associated `FluxHelmRelease`.

*/
package status

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/helm"

	"github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	fluxclientset "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	v1beta1client "github.com/weaveworks/flux/integrations/client/clientset/versioned/typed/flux.weave.works/v1beta1"
	"github.com/weaveworks/flux/integrations/helm/release"
)

const period = 10 * time.Second

type Updater struct {
	fluxhelm   fluxclientset.Interface
	kube       kube.Interface
	helmClient *helm.Client
}

func New(fhrClient fluxclientset.Interface, kubeClient kube.Interface, helmClient *helm.Client) *Updater {
	return &Updater{
		fluxhelm:   fhrClient,
		kube:       kubeClient,
		helmClient: helmClient,
	}
}

func (a *Updater) Loop(stop <-chan struct{}, logger log.Logger) {
	ticker := time.NewTicker(period)
	var logErr error

bail:
	for {
		select {
		case <-stop:
			break bail
		case <-ticker.C:
		}
		// Look up FluxHelmReleases
		namespaces, err := a.kube.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			logErr = err
			break bail
		}
		for _, ns := range namespaces.Items {
			fhrClient := a.fluxhelm.FluxV1beta1().FluxHelmReleases(ns.Name)
			fhrs, err := fhrClient.List(metav1.ListOptions{})
			if err != nil {
				logErr = err
				break bail
			}
			for _, fhr := range fhrs.Items {
				releaseName := release.GetReleaseName(fhr)
				// If we don't get the content, we don't care why
				content, _ := a.helmClient.ReleaseContent(releaseName)
				if content == nil {
					continue
				}
				status := content.GetRelease().GetInfo().GetStatus()
				if status.GetCode().String() != fhr.Status.ReleaseStatus {
					err := UpdateReleaseStatus(fhrClient, fhr, releaseName, status.GetCode().String())
					if err != nil {
						logger.Log("namespace", ns.Name, "resource", fhr.Name, "err", err)
						continue
					}
				}
			}
		}
	}

	ticker.Stop()
	logger.Log("loop", "stopping", "err", logErr)
}

func UpdateReleaseStatus(client v1beta1client.FluxHelmReleaseInterface, fhr v1beta1.FluxHelmRelease, releaseName, releaseStatus string) error {
	patchBytes, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"releaseName":   releaseName,
			"releaseStatus": releaseStatus,
		},
	})
	if err == nil {
		// CustomResources don't get
		// StrategicMergePatch, for now, but since we
		// want to unconditionally set the value, this
		// is OK.
		_, err = client.Patch(fhr.Name, types.MergePatchType, patchBytes)
	}
	return err
}
