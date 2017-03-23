package rpc

import (
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/platform"
)

type baseClient struct{}

var _ platform.Platform = baseClient{}

func (bc baseClient) AllServices(string, flux.ServiceIDSet) ([]platform.Service, error) {
	return nil, platform.UpgradeNeededError(errors.New("AllServices method not implemented"))
}

func (bc baseClient) SomeServices([]flux.ServiceID) ([]platform.Service, error) {
	return nil, platform.UpgradeNeededError(errors.New("SomeServices method not implemented"))
}

func (bc baseClient) Apply([]platform.ServiceDefinition) error {
	return platform.UpgradeNeededError(errors.New("Apply method not implemented"))
}

func (bc baseClient) Ping() error {
	return platform.UpgradeNeededError(errors.New("Ping method not implemented"))
}

func (bc baseClient) Version() (string, error) {
	return "", platform.UpgradeNeededError(errors.New("Version method not implemented"))
}

func (bc baseClient) Export() ([]byte, error) {
	return nil, platform.UpgradeNeededError(errors.New("Export method not implemented"))
}

func (bc baseClient) Sync(platform.SyncDef) error {
	return platform.UpgradeNeededError(errors.New("Sync method not implemented"))
}
