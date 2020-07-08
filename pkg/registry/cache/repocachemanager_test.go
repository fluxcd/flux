package cache

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/registry"
	zapLogfmt "github.com/sykesm/zap-logfmt"
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
	zap.RegisterEncoder("logfmt", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		enc := zapLogfmt.NewEncoder(config)
		return enc, nil
	})
	logCfg := zap.NewDevelopmentConfig()
	logCfg.Encoding = "logfmt"
	logger, _ := logCfg.Build()
	cf := &registry.RemoteClientFactory{
		Logger:        logger,
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
