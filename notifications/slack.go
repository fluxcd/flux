package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/update"
)

type SlackMsg struct {
	Username    string            `json:"username"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

type SlackAttachment struct {
	Fallback string   `json:"fallback,omitempty"`
	Text     string   `json:"text"`
	Author   string   `json:"author_name,omitempty"`
	Color    string   `json:"color,omitempty"`
	Markdown []string `json:"mrkdwn_in,omitempty"`
}

func errorAttachment(msg string) SlackAttachment {
	return SlackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "warning",
	}
}

func successAttachment(msg string) SlackAttachment {
	return SlackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "good",
	}
}

const (
	ReleaseTemplate = `Release {{trim (print .Release.Spec.ImageSpec) "<>"}} to {{with .Release.Spec.ServiceSpecs}}{{range $index, $spec := .}}{{if not (eq $index 0)}}, {{if last $index $.Release.Spec.ServiceSpecs}}and {{end}}{{end}}{{trim (print .) "<>"}}{{end}}{{end}}.`

	AutoReleaseTemplate = `Automated release of new image{{if not (last 0 $.Images)}}s{{end}} {{with .Images}}{{range $index, $image := .}}{{if not (eq $index 0)}}, {{if last $index $.Images}}and {{end}}{{end}}{{.}}{{end}}{{end}}.`
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func hasNotifyEvent(config service.NotifierConfig, event string) bool {
	// For backwards compatibility: if no such configuration exists,
	// assume we just care about releases and autoreleases
	if config.NotifyEvents == nil {
		return event == history.EventRelease || event == history.EventAutoRelease
	}
	for _, s := range config.NotifyEvents {
		if s == event {
			return true
		}
	}
	return false
}

func slackNotifyRelease(config service.NotifierConfig, release *history.ReleaseEventMetadata, releaseError string) error {
	if !hasNotifyEvent(config, history.EventRelease) {
		return nil
	}
	// Sanity check: we shouldn't get any other kind, but you
	// never know.
	if release.Spec.Kind != update.ReleaseKindExecute {
		return nil
	}
	var attachments []SlackAttachment

	text, err := instantiateTemplate("release", ReleaseTemplate, struct {
		Release *history.ReleaseEventMetadata
	}{
		Release: release,
	})
	if err != nil {
		return err
	}

	if releaseError != "" {
		attachments = append(attachments, errorAttachment(releaseError))
	}

	if release.Cause.User != "" || release.Cause.Message != "" {
		cause := SlackAttachment{}
		if user := release.Cause.User; user != "" {
			cause.Author = user
		}
		if msg := release.Cause.Message; msg != "" {
			cause.Text = msg
		}
		attachments = append(attachments, cause)
	}

	if release.Result != nil {
		result := slackResultAttachment(release.Result)
		attachments = append(attachments, result)
	}

	return notify(config, SlackMsg{
		Username:    config.Username,
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifyAutoRelease(config service.NotifierConfig, release *history.AutoReleaseEventMetadata, releaseError string) error {
	if !hasNotifyEvent(config, history.EventAutoRelease) {
		return nil
	}

	var attachments []SlackAttachment

	if releaseError != "" {
		attachments = append(attachments, errorAttachment(releaseError))
	}
	if release.Result != nil {
		attachments = append(attachments, slackResultAttachment(release.Result))
	}
	text, err := instantiateTemplate("auto-release", AutoReleaseTemplate, struct {
		Images []flux.ImageID
	}{
		Images: release.Spec.Images(),
	})
	if err != nil {
		return err
	}

	return notify(config, SlackMsg{
		Username:    config.Username,
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifySync(config service.NotifierConfig, sync *history.Event) error {
	if !hasNotifyEvent(config, history.EventSync) {
		return nil
	}

	details := sync.Metadata.(*history.SyncEventMetadata)
	// Only send a notification if this contains something other
	// releases and autoreleases (and we were told what it contains)
	if details.Includes != nil {
		if _, ok := details.Includes[history.NoneOfTheAbove]; !ok {
			return nil
		}
	}

	var attachments []SlackAttachment
	// A check to see if we got messages with our commits; older
	// versions don't send them.
	if len(details.Commits) > 0 && details.Commits[0].Message != "" {
		attachments = append(attachments, slackCommitsAttachment(details))
	}
	return notify(config, SlackMsg{
		Username:    config.Username,
		Text:        sync.String(),
		Attachments: attachments,
	})
}

func slackResultAttachment(res update.Result) SlackAttachment {
	buf := &bytes.Buffer{}
	update.PrintResults(buf, res, false)
	c := "good"
	if res.Error() != "" {
		c = "warning"
	}
	return SlackAttachment{
		Text:     "```" + buf.String() + "```",
		Markdown: []string{"text"},
		Color:    c,
	}
}

func slackCommitsAttachment(ev *history.SyncEventMetadata) SlackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")

	for i := range ev.Commits {
		fmt.Fprintf(buf, "%s %s\n", ev.Commits[i].Revision[:7], ev.Commits[i].Message)
	}
	fmt.Fprintln(buf, "```")
	return SlackAttachment{
		Text:     buf.String(),
		Markdown: []string{"text"},
		Color:    "good",
	}
}

func notify(config service.NotifierConfig, msg SlackMsg) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(msg); err != nil {
		return errors.Wrap(err, "encoding Slack POST request")
	}

	req, err := http.NewRequest("POST", config.HookURL, buf)
	if err != nil {
		return errors.Wrap(err, "constructing Slack HTTP request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to Slack")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return fmt.Errorf("%s from Slack (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func instantiateTemplate(tmplName, tmplStr string, args interface{}) (string, error) {
	tmpl, err := template.New(tmplName).Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
