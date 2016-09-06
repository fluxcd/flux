package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

func NewSlackEventWriter(d Doer, webhookURL, username string, matchExprs ...string) *Slack {
	var re []*regexp.Regexp
	for _, expr := range matchExprs {
		re = append(re, regexp.MustCompile(expr))
	}
	return &Slack{
		d:          d,
		webhookURL: webhookURL,
		username:   username,
		re:         re,
	}
}

type Slack struct {
	d          Doer
	webhookURL string
	username   string
	re         []*regexp.Regexp
}

func (s *Slack) LogEvent(namespace, service, msg string) error {
	text := fmt.Sprintf("%s/%s: %s", namespace, service, msg)
	if !s.match(text) {
		return nil
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(map[string]string{
		"username": s.username,
		"text":     text,
	}); err != nil {
		return errors.Wrap(err, "encoding Slack POST request")
	}

	req, err := http.NewRequest("POST", s.webhookURL, buf)
	if err != nil {
		return errors.Wrap(err, "constructing Slack HTTP request")
	}
	resp, err := s.d.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to Slack")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return fmt.Errorf("%s from Slack (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func (s *Slack) match(text string) bool {
	for _, re := range s.re {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// Doer is satisfied by *http.Client.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}
