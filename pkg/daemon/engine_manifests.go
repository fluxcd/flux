package daemon

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/engine/pkg"
	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/manifests"
	"github.com/ghodss/yaml"
)

func (s *engineSettings) getManifestStore(r repo) (manifests.Store, error) {
	absPaths := git.MakeAbsolutePaths(r, s.gitConfig.Paths)
	if s.manifestGenerationEnabled {
		return manifests.NewConfigAware(r.Dir(), absPaths, s.manifests)
	}
	return manifests.NewRawFiles(r.Dir(), absPaths, s.manifests), nil
}

func (s *engineSettings) Generate(ctx context.Context, repo *v1alpha1.Repository, revision string, source *v1alpha1.ApplicationSource, setting *pkg.ManifestGenerationSettings) (*pkg.ManifestResponse, error) {
	// Make a read-only clone used for this sync
	ctxt, cancel := context.WithTimeout(ctx, s.gitTimeout)
	working, err := s.repo.Export(ctxt, revision)
	if err != nil {
		return nil, err
	}
	cancel()
	defer working.Clean()

	// Unseal any secrets if enabled
	if s.gitSecretEnabled {
		ctxt, cancel := context.WithTimeout(ctx, s.gitTimeout)
		if err := working.SecretUnseal(ctxt); err != nil {
			return nil, err
		}
		cancel()
	}
	resourceStore, err := s.getManifestStore(working)
	if err != nil {
		return nil, fmt.Errorf("reading the repository checkout: %v", err)
	}
	ctxt, cancel = context.WithTimeout(ctx, s.gitTimeout)
	revision, err = s.repo.Revision(ctxt, revision)
	if err != nil {
		return nil, err
	}
	cancel()

	resources, err := resourceStore.GetAllResourcesByID(ctx)

	mfst := make([]string, 0)
	for i := range resources {
		data, err := yaml.YAMLToJSON(resources[i].Bytes())
		if err != nil {
			return nil, err
		}
		mfst = append(mfst, string(data))
	}
	return &pkg.ManifestResponse{
		Namespace: s.namespace,
		Revision:  revision,
		Manifests: mfst,
	}, nil
}
