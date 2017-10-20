package notifications

import (
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/service/instance"
)

var DefaultNotifyEvents = []string{event.EventRelease, event.EventAutoRelease}

func Event(cfg instance.Config, e event.Event) error {
	// If this is a release
	if cfg.Settings.Slack.HookURL != "" {
		switch e.Type {
		case event.EventRelease:
			r := e.Metadata.(*event.ReleaseEventMetadata)
			return slackNotifyRelease(cfg.Settings.Slack, r, r.Error)
		case event.EventAutoRelease:
			r := e.Metadata.(*event.AutoReleaseEventMetadata)
			return slackNotifyAutoRelease(cfg.Settings.Slack, r, r.Error)
		case event.EventSync:
			return slackNotifySync(cfg.Settings.Slack, &e)
		}
	}
	return nil
}
