package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

const (
	defaultReleaseTemplate = `Release {{trim (print .Spec.ImageSpec) "<>"}} to {{with .Spec.ServiceSpecs}}{{range $index, $spec := .}}{{if not (eq $index 0)}}, {{if last $index $}}and {{end}}{{end}}{{trim (print .) "<>"}}{{end}}{{end}}. {{with .Error}}{{.}}. failed{{else}}done{{end}}`
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func slackNotifyRelease(config flux.NotifierConfig, release flux.Release, releaseError error) error {
	if release.Spec.Kind == flux.ReleaseKindPlan {
		return nil
	}

	template := defaultReleaseTemplate
	if config.ReleaseTemplate != "" {
		template = config.ReleaseTemplate
	}

	errorMessage := ""
	if releaseError != nil {
		errorMessage = releaseError.Error()
	}
	text, err := instantiateTemplate("release", template, struct {
		flux.Release
		Error string
	}{
		Release: release,
		Error:   errorMessage,
	})
	if err != nil {
		return err
	}

	return notify(config, text)
}

func notify(config flux.NotifierConfig, text string) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(map[string]string{
		"username": config.Username,
		"text":     text,
	}); err != nil {
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
	tmpl, err := template.New(tmplName).Funcs(template.FuncMap{
		"iso8601":    func(t time.Time) string { return t.Format(time.RFC3339) },
		"join":       strings.Join,
		"replace":    strings.Replace,
		"trim":       strings.Trim,
		"trimLeft":   strings.TrimLeft,
		"trimPrefix": strings.TrimPrefix,
		"trimRight":  strings.TrimRight,
		"trimSuffix": strings.TrimSuffix,
		"trimSpace":  strings.TrimSpace,
		"last":       func(i int, a interface{}) bool { return i == reflect.ValueOf(a).Len()-1 },
	}).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
