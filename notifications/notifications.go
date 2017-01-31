package notifications

import (
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/instance"
)

// Release performs post-release notifications for an instance
func Release(cfg instance.Config, r flux.Release, releaseError error) error {
	// TODO: Use a config settings format which allows multiple notifiers to be
	// configured.
	var err error
	if cfg.Settings.Slack.HookURL != "" {
		err = slackNotifyRelease(cfg.Settings.Slack, r, releaseError)
	}
	return err
}
