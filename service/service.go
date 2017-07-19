package service

import (
	"time"

	"github.com/weaveworks/flux"
)

type InstanceID string

const NoInstanceID InstanceID = "NoInstanceID"

const InstanceIDHeaderKey = "X-Scope-OrgID"

// TODO: How similar should this be to the `get-config` result?
type Status struct {
	Fluxsvc FluxsvcStatus `json:"fluxsvc" yaml:"fluxsvc"`
	Fluxd   FluxdStatus   `json:"fluxd" yaml:"fluxd"`
	Git     GitStatus     `json:"git" yaml:"git"`
}

type FluxsvcStatus struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

type FluxdStatus struct {
	Connected bool      `json:"connected" yaml:"connected"`
	Last      time.Time `json:"last,omitempty" yaml:"last,omitempty"`
	Version   string    `json:"version,omitempty" yaml:"version,omitempty"`
}

type GitStatus struct {
	Configured bool           `json:"configured" yaml:"configured"`
	Error      string         `json:"error,omitempty" yaml:"error,omitempty"`
	Config     flux.GitConfig `json:"config"`
}
