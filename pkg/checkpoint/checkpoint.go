package checkpoint

import (
	"time"

	"github.com/go-kit/kit/log"
)

const (
	versionCheckPeriod = 6 * time.Hour
)

func CheckForUpdates(product, version string, extra map[string]string, logger log.Logger) *checker {
	handleResponse := func() {
		logger.Log("msg", "Flux v1 is deprecated, please upgrade to v2", "latest", "v2", "URL", "https://fluxcd.io/docs/migration/")
	}

	flags := map[string]string{
		"kernel-version": "XXXXX",
	}
	for k, v := range extra {
		flags[k] = v
	}

	params := checkParams{
		Product:       product,
		Version:       version,
		SignatureFile: "",
		Flags:         flags,
	}

	return checkInterval(&params, versionCheckPeriod, handleResponse)
}

func checkInterval(p *checkParams, interval time.Duration,
	cb func()) *checker {

	state := &checker{
		doneCh: make(chan struct{}),
	}

	if isCheckDisabled() {
		return state
	}

	go func() {
		cb()

		for {
			after := randomStagger(interval)
			state.nextCheckAtLock.Lock()
			state.nextCheckAt = time.Now().Add(after)
			state.nextCheckAtLock.Unlock()

			select {
			case <-time.After(after):
				cb()
			case <-state.doneCh:
				return
			}
		}
	}()

	return state
}
