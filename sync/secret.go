package sync

import (
	"context"
	"encoding/json"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

const syncMarkerKey = "flux.weave.works/sync-hwm"

// NativeSyncProvider keeps information related to the native state of a sync marker stored in a "native" kubernetes resource.
type NativeSyncProvider struct {
	namespace    string
	revision     string
	resourceName string
	resourceAPI  v1.SecretInterface
}

// NewNativeSyncProvider creates a new NativeSyncProvider
func NewNativeSyncProvider(namespace string, resourceName string) (NativeSyncProvider, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return NativeSyncProvider{}, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return NativeSyncProvider{}, err
	}

	return NativeSyncProvider{
		resourceAPI:  clientset.CoreV1().Secrets(namespace),
		namespace:    namespace,
		resourceName: resourceName,
	}, nil
}

func (p NativeSyncProvider) String() string {
	return "kubernetes " + p.namespace + ":secret/" + p.resourceName
}

// GetRevision gets the revision of the current sync marker (representing the place flux has synced to).
func (p NativeSyncProvider) GetRevision(ctx context.Context) (string, error) {
	resource, err := p.resourceAPI.Get(p.resourceName, meta_v1.GetOptions{})
	if err != nil {
		return "", err
	}
	revision, exists := resource.Annotations[syncMarkerKey]
	if !exists {
		return "", p.setRevision("")
	}
	return revision, nil
}

// UpdateMarker updates the revision the sync marker points to.
func (p NativeSyncProvider) UpdateMarker(ctx context.Context, revision string) error {
	return p.setRevision(revision)
}

// DeleteMarker resets the state of the object.
func (p NativeSyncProvider) DeleteMarker(ctx context.Context) error {
	return p.setRevision("")
}

func (p NativeSyncProvider) setRevision(revision string) error {
	jsonPatch, err := json.Marshal(patch(revision))
	if err != nil {
		return err
	}

	_, err = p.resourceAPI.Patch(
		p.resourceName,
		types.StrategicMergePatchType,
		jsonPatch,
	)
	return err
}

func patch(revision string) map[string]map[string]map[string]string {
	return map[string]map[string]map[string]string{
		"metadata": map[string]map[string]string{
			"annotations": map[string]string{
				syncMarkerKey: revision,
			},
		},
	}
}
