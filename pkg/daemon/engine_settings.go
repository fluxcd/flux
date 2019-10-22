package daemon

import (
	"context"
	"errors"

	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/resource"
)

func (s *engineSettings) GetAppInstanceLabelKey() (string, error) {
	return "fluxcd.io/application", nil
}

func (s *engineSettings) GetResourcesFilter() (*resource.ResourcesFilter, error) {
	return &resource.ResourcesFilter{}, nil
}

func (s *engineSettings) GetResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return make(map[string]v1alpha1.ResourceOverride), nil
}

func (s *engineSettings) GetConfigManagementPlugins() ([]v1alpha1.ConfigManagementPlugin, error) {
	return nil, nil
}
func (s *engineSettings) GetKustomizeBuildOptions() (string, error) {
	return "", nil
}

func (s *engineSettings) Subscribe(subCh chan<- bool) {

}

func (s *engineSettings) Unsubscribe(subCh chan<- bool) {

}

func (s *engineSettings) GetCluster(ctx context.Context, name string) (*v1alpha1.Cluster, error) {
	return &v1alpha1.Cluster{Server: clusterURL}, nil
}

func (s *engineSettings) WatchClusters(ctx context.Context, callback func(event *pkg.ClusterEvent)) error {
	return nil
}
func (s *engineSettings) ListHelmRepositories(ctx context.Context) ([]*v1alpha1.Repository, error) {
	return nil, nil
}

func (s *engineSettings) GetRepository(ctx context.Context, url string) (*v1alpha1.Repository, error) {
	return &v1alpha1.Repository{Repo: url}, nil
}

func (s *engineSettings) LogAppEvent(app *v1alpha1.Application, info pkg.EventInfo, message string) {
}

func (s *engineSettings) SetAppResourcesTree(appName string, resourcesTree *v1alpha1.ApplicationTree) error {
	return nil
}

func (s *engineSettings) SetAppManagedResources(appName string, managedResources []*v1alpha1.ResourceDiff) error {
	return nil
}

func (s *engineSettings) GetAppManagedResources(appName string, res *[]*v1alpha1.ResourceDiff) error {
	return errors.New("not supported")
}
