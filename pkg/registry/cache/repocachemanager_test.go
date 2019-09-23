package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
)

func Test_ClientTimeouts(t *testing.T) {
	timeout := 1 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		// make sure we exceed the timeout
		time.Sleep(timeout * 10)
	}))
	defer server.Close()
	url, err := url.Parse(server.URL)
	assert.NoError(t, err)
	logger := log.NewLogfmtLogger(os.Stdout)
	cf := &registry.RemoteClientFactory{
		Logger:        log.NewLogfmtLogger(os.Stdout),
		Limiters:      nil,
		Trace:         false,
		InsecureHosts: []string{url.Host},
	}
	name := image.Name{
		Domain: url.Host,
		Image:  "foo/bar",
	}
	rcm, err := newRepoCacheManager(
		time.Now(),
		name,
		cf,
		registry.NoCredentials(),
		timeout,
		1,
		false,
		logger,
		nil,
	)
	assert.NoError(t, err)
	_, err = rcm.getTags(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "client timeout (1ms) exceeded", err.Error())
}
