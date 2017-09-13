package automator

import (
	"errors"
	"strings"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux/instance"
	"github.com/weaveworks/flux/jobs"
)

// Config collects the parameters to the automator. All fields are mandatory.
type Config struct {
	Jobs       jobs.JobReadPusher
	InstanceDB instance.DB
	Instancer  instance.Instancer
	Period     time.Duration
	Logger     log.Logger
}

// Validate returns an error if the config is underspecified.
func (cfg Config) Validate() error {
	var errs []string
	if cfg.Jobs == nil {
		errs = append(errs, "job queue not supplied")
	}
	if cfg.InstanceDB == nil {
		errs = append(errs, "instance configuration DB not supplied")
	}
	if cfg.Logger == nil {
		errs = append(errs, "logger not supplied")
	}
	if cfg.Period == 0 {
		errs = append(errs, "automation period not supplied")
	}
	if len(errs) > 0 {
		return errors.New("invalid: " + strings.Join(errs, "; "))
	}
	return nil
}
