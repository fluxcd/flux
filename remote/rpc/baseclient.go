package rpc

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

type baseClient struct{}

var _ remote.Platform = baseClient{}

func (bc baseClient) Version() (string, error) {
	return "", remote.UpgradeNeededError(errors.New("Version method not implemented"))
}

func (bc baseClient) Ping() error {
	return remote.UpgradeNeededError(errors.New("Ping method not implemented"))
}

func (bc baseClient) Export() ([]byte, error) {
	return nil, remote.UpgradeNeededError(errors.New("Export method not implemented"))
}

func (bc baseClient) ListServices(string) ([]flux.ServiceStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListServices method not implemented"))
}

func (bc baseClient) ListImages(update.ServiceSpec) ([]flux.ImageStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListImages method not implemented"))
}

func (bc baseClient) UpdateManifests(update.Spec) (job.ID, error) {
	var id job.ID
	return id, remote.UpgradeNeededError(errors.New("UpdateManifests method not implemented"))
}

func (bc baseClient) SyncNotify() error {
	return remote.UpgradeNeededError(errors.New("SyncNotify method not implemented"))
}

func (bc baseClient) JobStatus(job.ID) (job.Status, error) {
	return job.Status{}, remote.UpgradeNeededError(errors.New("JobStatus method not implemented"))
}

func (bc baseClient) SyncStatus(string) ([]string, error) {
	return nil, remote.UpgradeNeededError(errors.New("SyncStatus method not implemented"))
}
