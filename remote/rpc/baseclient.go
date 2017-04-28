package rpc

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/remote"
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

func (bc baseClient) ListImages(flux.ServiceSpec) ([]flux.ImageStatus, error) {
	return nil, remote.UpgradeNeededError(errors.New("ListImages method not implemented"))
}

func (bc baseClient) UpdateImages(flux.ReleaseSpec) (flux.ReleaseResult, error) {
	return nil, remote.UpgradeNeededError(errors.New("UpdateImages method not implemented"))
}

func (bc baseClient) SyncNotify() error {
	return remote.UpgradeNeededError(errors.New("SyncNotify method not implemented"))
}

func (bc baseClient) SyncStatus(string) ([]string, error) {
	return nil, remote.UpgradeNeededError(errors.New("SyncStatus method not implemented"))
}

func (bc baseClient) UpdatePolicies(flux.PolicyUpdates) error {
	return remote.UpgradeNeededError(errors.New("UpdatePolicies method not implemented"))
}
