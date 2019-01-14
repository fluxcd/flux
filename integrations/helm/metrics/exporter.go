package metrics

import (
	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	"time"
)

type Exporter struct {
	logger log.Logger
	client *helm.Client
	status *prometheus.GaugeVec
}

func NewExporter(logger log.Logger, client *helm.Client, register bool) *Exporter {
	status := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "helm_release_status",
		Help: "Helm release install status",
	}, []string{"release", "chart", "version", "namespace"})

	if register {
		prometheus.MustRegister(status)
	}

	exporter := &Exporter{
		logger: logger,
		client: client,
		status: status,
	}

	return exporter
}

func statusCodes() []release.Status_Code {
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
	return []release.Status_Code{
		release.Status_UNKNOWN,
		release.Status_DEPLOYED,
		release.Status_DELETED,
		release.Status_DELETING,
		release.Status_FAILED,
		release.Status_PENDING_INSTALL,
		release.Status_PENDING_UPGRADE,
		release.Status_PENDING_ROLLBACK,
	}
}

// filterList returns a list scrubbed of old releases.
// source: https://github.com/helm/helm/blob/master/cmd/helm/list.go#L197
func filterList(rels []*release.Release) []*release.Release {
	idx := map[string]int32{}

	for _, r := range rels {
		name, version := r.GetName(), r.GetVersion()
		if max, ok := idx[name]; ok {
			// check if we have a greater version already
			if max > version {
				continue
			}
		}
		idx[name] = version
	}

	uniq := make([]*release.Release, 0, len(idx))
	for _, r := range rels {
		if idx[r.GetName()] == r.GetVersion() {
			uniq = append(uniq, r)
		}
	}
	return uniq
}

func (e *Exporter) getStats() {
	releases, err := e.client.ListReleases(helm.ReleaseListStatuses(statusCodes()))
	if err != nil {
		e.logger.Log("error", err.Error())
		return
	}

	e.status.Reset()
	for _, rel := range filterList(releases.GetReleases()) {
		releaseName := rel.GetName()
		chart := rel.GetChart().GetMetadata().GetName()
		version := rel.GetChart().GetMetadata().GetVersion()
		namespace := rel.GetNamespace()
		status := rel.GetInfo().GetStatus().GetCode()
		e.status.WithLabelValues(releaseName, chart, version, namespace).Set(float64(status))
	}
}

func (e *Exporter) Run(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			e.getStats()
		case <-stopCh:
			return
		}
	}
}
