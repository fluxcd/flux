package notifications

import (
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/update"
)

// Release performs post-release notifications for an instance
func Release(cfg instance.Config, r *history.ReleaseEventMetadata, releaseError string) error {
	if r.Spec.Kind != update.ReleaseKindExecute {
		return nil
	}

	// TODO: Use a config settings format which allows multiple notifiers to be
	// configured.
	var err error
	if cfg.Settings.Slack.HookURL != "" {
		err = slackNotifyRelease(cfg.Settings.Slack, r, releaseError)
	}
	return err
}
