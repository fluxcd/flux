/*

This package is for maintaining the link between `FluxHelmRelease`
resources and the Helm releases to which they
correspond. Specifically,

 1. updating the `FluxHelmRelease` status based on the state of the
   associated Helm release; and,

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

	fluxhelmtypes "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	fluxhelm "github.com/weaveworks/flux/integrations/client/clientset/versioned"
	"github.com/weaveworks/flux/integrations/helm/release"
)

const period = 10 * time.Second

type Updater struct {
	fluxhelm   fluxhelm.Interface
	kube       kube.Interface
	helmClient *helm.Client
}

func New(fhrClient fluxhelm.Interface, kubeClient kube.Interface, helmClient *helm.Client) *Updater {
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
			fhrIf := a.fluxhelm.FluxV1beta1().FluxHelmReleases(ns.Name)
			fhrs, err := fhrIf.List(metav1.ListOptions{})
			if err != nil {
				logErr = err
				break bail
			}
			for _, fhr := range fhrs.Items {
				releaseName := release.GetReleaseName(fhr)
				content, err := a.helmClient.ReleaseContent(releaseName)
				if err != nil {
					logger.Log("err", err)
					continue
				}
				status := content.GetRelease().GetInfo().GetStatus()
				if status.GetCode().String() != fhr.Status.ReleaseStatus {
					newStatus := fluxhelmtypes.FluxHelmReleaseStatus{
						ReleaseStatus: status.GetCode().String(),
					}
					var patchBytes []byte
					if patchBytes, err = json.Marshal(map[string]interface{}{
						"status": newStatus,
					}); err == nil {
						// CustomResources don't get
						// StrategicMergePatch, for now, but since we
						// want to unconditionally set the value, this
						// is OK.
						_, err = fhrIf.Patch(fhr.Name, types.MergePatchType, patchBytes)
					}
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
