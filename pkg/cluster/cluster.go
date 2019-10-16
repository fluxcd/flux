package cluster

import (
	"context"
	"errors"

	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/ssh"
)

// Constants for workload ready status. These are defined here so that
// no-one has to drag in Kubernetes dependencies to be able to use
// them.
const (
	StatusUnknown  = "unknown"
	StatusError    = "error"
	StatusReady    = "ready"
	StatusUpdating = "updating"
	StatusStarted  = "started"
)

// The things we can get from the running cluster. These used to form
// the remote.Platform interface; but now we do more in the daemon so they
// are distinct interfaces.
type Cluster interface {
	// Get all of the services (optionally, from a specific namespace), excluding those
	AllWorkloads(ctx context.Context, maybeNamespace string) ([]Workload, error)
	SomeWorkloads(ctx context.Context, ids []resource.ID) ([]Workload, error)
	IsAllowedResource(resource.ID) bool
	Ping() error
	Export(ctx context.Context) ([]byte, error)
	Sync(SyncSet) error
	PublicSSHKey(regenerate bool) (ssh.PublicKey, error)
}

// RolloutStatus describes numbers of pods in different states and
// the messages about unexpected rollout progress
// a rollout status might be:
// - in progress: Updated, Ready or Available numbers are not equal to Desired, or Outdated not equal to 0
// - stuck: Messages contains info if deployment unavailable or exceeded its progress deadline
// - complete: Updated, Ready and Available numbers are equal to Desired and Outdated equal to 0
// See https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#deployment-status
type RolloutStatus struct {
	// Desired number of pods as defined in spec.
	Desired int32
	// Updated number of pods that are on the desired pod spec.
	Updated int32
	// Ready number of pods targeted by this deployment.
	Ready int32
	// Available number of available pods (ready for at least minReadySeconds) targeted by this deployment.
	Available int32
	// Outdated number of pods that are on a different pod spec.
	Outdated int32
	// Messages about unexpected rollout progress
	// if there's a message here, the rollout will not make progress without intervention
	Messages []string
}

// Workload describes a cluster resource that declares versioned images.
type Workload struct {
	ID     resource.ID
	Status string // A status summary for display
	// Is the controller considered read-only because it's under the
	// control of the platform. In the case of Kubernetes, we simply
	// omit these controllers; but this may not always be the case.
	IsSystem bool
	// If this workload was created _because_ of another, antecedent
	// resource through some mechanism (like an operator, or custom
	// resource controller), we try to record the ID of that resource
	// in this field.
	Antecedent resource.ID
	Labels     map[string]string
	Policies   policy.Set
	Rollout    RolloutStatus
	// Errors during the recurring sync from the Git repository to the
	// cluster will surface here.
	SyncError error

	Containers ContainersOrExcuse
}

// Sometimes we care if we can't find the containers for a service,
// sometimes we just want the information we can get.
type ContainersOrExcuse struct {
	Excuse     string
	Containers []resource.Container
}

func (s Workload) ContainersOrNil() []resource.Container {
	return s.Containers.Containers
}

func (s Workload) ContainersOrError() ([]resource.Container, error) {
	var err error
	if s.Containers.Excuse != "" {
		err = errors.New(s.Containers.Excuse)
	}
	return s.Containers.Containers, err
}
