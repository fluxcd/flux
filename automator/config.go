package automator

import (
	"errors"
	"os"
	"strings"

	"github.com/weaveworks/fluxy/git"
	"github.com/weaveworks/fluxy/history"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Config collects the parameters to the automator. All fields are mandatory.
//
// This struct is threaded, coarsely, through automator components. This is an
// explicit decision decision, because the final shape and semantics of the
// automator will likely change significantly, and I'd like to keep API churn
// down.
type Config struct {
	// A reference to the orchestration platform.
	Platform *kubernetes.Cluster

	// A reference to the image registry.
	Registry *registry.Client

	// A reference to the audit history component.
	History history.DB

	// Repo is the git repo we are managing.
	Repo git.Repo
}

// Validate returns an error if the config is underspecified.
func (cfg Config) Validate() error {
	var errs []string
	if cfg.Platform == nil {
		errs = append(errs, "platform not specified")
	}
	if cfg.Registry == nil {
		errs = append(errs, "registry not specified")
	}
	if cfg.History == nil {
		errs = append(errs, "history not specified")
	}
	if cfg.Repo.URL == "" {
		errs = append(errs, "config repo URL not specified")
	}
	if len(cfg.Repo.Key) == 0 {
		errs = append(errs, "config repo key not specified")
	}
	if _, err := os.Stat(cfg.Repo.Key); err != nil {
		errs = append(errs, err.Error())
	}
	if cfg.Repo.Path == "" {
		errs = append(errs, "config repo path not specified")
	}
	if len(errs) > 0 {
		return errors.New("invalid: " + strings.Join(errs, "; "))
	}
	return nil
}
