package instance

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"
)

type MultitenantInstancer struct {
	DB                  DB
	Connecter           platform.Connecter
	Logger              log.Logger
	History             history.DB
	MemcacheClient      registry.MemcacheClient
	RegistryCacheExpiry time.Duration
}

func (m *MultitenantInstancer) Get(instanceID flux.InstanceID) (*Instance, error) {
	c, err := m.DB.GetConfig(instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "getting instance config from DB")
	}

	// Platform interface for this instance
	platform, err := m.Connecter.Connect(instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "connecting to platform")
	}

	// Logger specialised to this instance
	instanceLogger := log.NewContext(m.Logger).With("instanceID", instanceID)

	// Registry client with instance's config
	creds, err := registry.CredentialsFromConfig(c.Settings)
	if err != nil {
		return nil, errors.Wrap(err, "decoding registry credentials")
	}
	registryLogger := log.NewContext(instanceLogger).With("component", "registry")
	reg := registry.NewRegistry(
		registry.NewRemoteClientFactory(creds, registryLogger, m.MemcacheClient, m.RegistryCacheExpiry),
		registryLogger,
	)
	reg = registry.NewInstrumentedRegistry(reg)

	repo := gitRepoFromSettings(c.Settings)

	// Events for this instance
	eventRW := EventReadWriter{instanceID, m.History}

	// Configuration for this instance
	config := configurer{instanceID, m.DB}

	return New(
		platform,
		reg,
		config,
		repo,
		instanceLogger,
		eventRW,
		eventRW,
	), nil
}

func gitRepoFromSettings(settings flux.UnsafeInstanceConfig) git.Repo {
	branch := settings.Git.Branch
	if branch == "" {
		branch = "master"
	}
	return git.Repo{
		URL:    settings.Git.URL,
		Branch: branch,
		Key:    settings.Git.Key,
		Path:   settings.Git.Path,
	}
}
