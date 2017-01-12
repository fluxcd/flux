package instance

import (
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/platform"
	"github.com/weaveworks/flux/registry"
)

type MultitenantInstancer struct {
	DB              DB
	Connecter       platform.Connecter
	Logger          log.Logger
	Histogram       metrics.Histogram
	History         history.DB
	RegistryMetrics registry.Metrics
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
	regClient := &registry.Client{
		Credentials: creds,
		Logger:      log.NewContext(instanceLogger).With("component", "registry"),
		Metrics:     m.RegistryMetrics.WithInstanceID(instanceID),
	}

	repo := gitRepoFromSettings(c.Settings)

	// Events for this instance
	eventRW := EventReadWriter{instanceID, m.History}
	var eventW history.EventWriter = eventRW
	if c.Settings.Slack.HookURL != "" {
		eventW = history.TeeWriter(eventRW, history.NewSlackEventWriter(
			http.DefaultClient,
			c.Settings.Slack.HookURL,
			c.Settings.Slack.Username,
			`(done|failed|\(no result expected\))$`, // only catch the final message, or started msg for async releases
		))
	}

	// Configuration for this instance
	config := configurer{instanceID, m.DB}

	return New(
		platform,
		regClient,
		config,
		repo,
		instanceLogger,
		m.Histogram,
		eventRW,
		eventW,
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
