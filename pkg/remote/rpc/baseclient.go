package rpc

import (
	"context"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/api/v10"
	"github.com/fluxcd/flux/pkg/api/v11"
	"github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/api/v9"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/remote"
	"github.com/fluxcd/flux/pkg/update"
)

type baseClient struct{}

var _ api.Server = baseClient{}

func (bc baseClient) Version(context.Context) (string, error) {
	return "", remote.UpgradeNeededError(errors.New("Version method not implemented"))
}

func (bc baseClient) Ping(context.Context) error {
	return remote.UpgradeNeededError(errors.New("Ping method not implemented"))
}

func (bc baseClient) Export(context.Context) ([]byte, error) {
	return nil, remote.UpgradeNeededError(errors.New("Export method not implemented"))
}

func (bc baseClient) ListServices(context.Context, string) ([]v6.ControllerStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListServices method not implemented"))
}

func (bc baseClient) ListServicesWithOptions(context.Context, v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListServicesWithOptions method not implemented"))
}

func (bc baseClient) ListImages(context.Context, update.ResourceSpec) ([]v6.ImageStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListImages method not implemented"))
}

func (bc baseClient) ListImagesWithOptions(context.Context, v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListImagesWithOptions method not implemented"))
}

func (bc baseClient) UpdateManifests(context.Context, update.Spec) (job.ID, error) {
	var id job.ID
	return id, remote.UpgradeNeededError(errors.New("UpdateManifests method not implemented"))
}

func (bc baseClient) NotifyChange(context.Context, v9.Change) error {
	return remote.UpgradeNeededError(errors.New("NotifyChange method not implemented"))
}

func (bc baseClient) JobStatus(context.Context, job.ID) (job.Status, error) {
	return job.Status{}, remote.UpgradeNeededError(errors.New("JobStatus method not implemented"))
}

func (bc baseClient) SyncStatus(context.Context, string) ([]string, error) {
	return nil, remote.UpgradeNeededError(errors.New("SyncStatus method not implemented"))
}

func (bc baseClient) GitRepoConfig(context.Context, bool) (v6.GitConfig, error) {
	return v6.GitConfig{}, remote.UpgradeNeededError(errors.New("GitRepoConfig method not implemented"))
}
