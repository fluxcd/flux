package notifications

import (
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/service/instance"
)

var DefaultNotifyEvents = []string{"release", "autorelease"}

func Event(cfg instance.Config, e history.Event) error {
	// If this is a release
	if cfg.Settings.Slack.HookURL != "" {
		switch e.Type {
		case history.EventRelease:
			r := e.Metadata.(*history.ReleaseEventMetadata)
			return slackNotifyRelease(cfg.Settings.Slack, r, r.Error)
		case history.EventAutoRelease:
			r := e.Metadata.(*history.AutoReleaseEventMetadata)
			return slackNotifyAutoRelease(cfg.Settings.Slack, r, r.Error)
		case history.EventSync:
			return slackNotifySync(cfg.Settings.Slack, &e)
		}
	}
	return nil
}
