package automator

import (
	"sync"

	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

const (
	automationEnabled  = "Automation enabled."
	automationDisabled = "Automation disabled."
)

// Automator orchestrates continuous deployment for specific services.
type Automator struct {
	k8s *kubernetes.Cluster
	reg *registry.Client
	his history.DB

	mtx    sync.RWMutex
	active map[namespacedService]*svc
}

// New creates a new automator, sitting on top of the platform and registry.
func New(k8s *kubernetes.Cluster, reg *registry.Client, his history.DB) *Automator {
	return &Automator{
		k8s: k8s,
		reg: reg,
		his: his,

		active: map[namespacedService]*svc{},
	}
}

// Enable turns on automated (continuous) deployment for the named service. This
// call always succeeds; if the named service cannot be automated for some
// reason, that will be detected and happen autonomously.
func (a *Automator) Enable(namespace, serviceName string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	ns := namespacedService{namespace, serviceName}
	if _, ok := a.active[ns]; ok {
		return
	}

	onDelete := func() { a.deleteCallback(namespace, serviceName) }
	svcLogger := makeServiceLogger(a.his, namespace, serviceName)
	s := newSvc(namespace, serviceName, a.k8s, a.reg, svcLogger, onDelete)
	a.active[ns] = s

	a.his.LogEvent(namespace, serviceName, automationEnabled)
}

// Disable turns off automated (continuous) deployment for the named service.
// This is more of a signal; it may take some time for the service to be
// properly disabled.
func (a *Automator) Disable(namespace, serviceName string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	ns := namespacedService{namespace, serviceName}
	s, ok := a.active[ns]
	if !ok {
		return
	}

	// We signal delete rather than actually deleting anything here,
	// to make sure svc termination follows a single code path.
	s.signalDelete()

	a.his.LogEvent(namespace, serviceName, automationDisabled)
}

// deleteCallback is invoked by a svc when it shuts down. A svc may terminate
// itself, and so needs this as a form of accounting.
func (a *Automator) deleteCallback(namespace, serviceName string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	ns := namespacedService{namespace, serviceName}
	delete(a.active, ns)
}

type namespacedService struct {
	namespace string
	service   string
}
