package main

import (
	"time"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/go-checkpoint"
)

const (
	versionCheckPeriod = 6 * time.Hour
)

func checkForUpdates(clusterString string, gitString string, logger log.Logger) *checkpoint.Checker {
	handleResponse := func(r *checkpoint.CheckResponse, err error) {
		if err != nil {
			logger.Log("err", err)
			return
		}
		if r.Outdated {
			logger.Log("msg", "update available", "latest", r.CurrentVersion, "URL", r.CurrentDownloadURL)
			return
		}
		logger.Log("msg", "up to date", "latest", r.CurrentVersion)
	}

	flags := map[string]string{
		"kernel-version":  getKernelVersion(),
		"cluster-version": clusterString,
		"git-configured":  gitString,
	}
	params := checkpoint.CheckParams{
		Product:       "weave-flux",
		Version:       version,
		SignatureFile: "",
		Flags:         flags,
	}

	return checkpoint.CheckInterval(&params, versionCheckPeriod, handleResponse)
}
