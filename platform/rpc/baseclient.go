package rpc

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

type baseClient struct{}

var _ platform.Platform = baseClient{}

func (bc baseClient) Version() (string, error) {
	return "", platform.UpgradeNeededError(errors.New("Version method not implemented"))
}

func (bc baseClient) Ping() error {
	return platform.UpgradeNeededError(errors.New("Ping method not implemented"))
}

func (bc baseClient) Export() ([]byte, error) {
	return nil, platform.UpgradeNeededError(errors.New("Export method not implemented"))
}

func (bc baseClient) ListServices(string) ([]flux.ServiceStatus, error) {
	return nil, platform.UpgradeNeededError(errors.New("ListServices method not implemented"))
}

func (bc baseClient) ListImages(flux.ServiceSpec) ([]flux.ImageStatus, error) {
	return nil, platform.UpgradeNeededError(errors.New("ListImages method not implemented"))
}

func (bc baseClient) UpdateImages(flux.ReleaseSpec) (flux.ReleaseResult, error) {
	return nil, platform.UpgradeNeededError(errors.New("UpdateImages method not implemented"))
}

func (bc baseClient) SyncCluster() error {
	return platform.UpgradeNeededError(errors.New("SyncCluster method not implemented"))
}

func (bc baseClient) SyncStatus(string) ([]string, error) {
	return nil, platform.UpgradeNeededError(errors.New("SyncStatus method not implemented"))
}
