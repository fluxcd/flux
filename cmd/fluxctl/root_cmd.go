package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/client"
	transport "github.com/weaveworks/fluxy/http"
)

type rootOpts struct {
	URL     string
	Token   string
	FluxSVC client.Client
}

// fluxctl never sends an instance ID directly; it's always blank, and
// optionally gets populated by an intermediating authfe from the token.
const noInstanceID = flux.InstanceID("")

type serviceOpts struct {
	*rootOpts
}

func newService(parent *rootOpts) *serviceOpts {
	return &serviceOpts{rootOpts: parent}
}

func newRoot() *rootOpts {
	return &rootOpts{}
}

var rootLongHelp = strings.TrimSpace(`
fluxctl helps you deploy your code.

Workflow:
  fluxctl list-services                                        # Which services are running?
  fluxctl list-images --service=default/foo                    # Which images are running/available?
  fluxctl release --service=default/foo --update-image=bar:v2  # Release new version.
  fluxctl history --service=default/foo                        # Review what happened
`)

const envVariableURL = "FLUX_URL"

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Long:              rootLongHelp,
		SilenceUsage:      true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "http://localhost:3030",
		fmt.Sprintf("base URL of the fluxd API server; you can also set the environment variable %s", envVariableURL),
	)
	cmd.PersistentFlags().StringVarP(&opts.Token, "token", "t", "",
		"Weave Cloud token",
	)

	svcopts := newService(opts)

	cmd.AddCommand(
		newServiceShow(svcopts).Command(),
		newServiceList(svcopts).Command(),
		newServiceRelease(svcopts).Command(),
		newServiceCheckRelease(svcopts).Command(),
		newServiceHistory(svcopts).Command(),
		newServiceAutomate(svcopts).Command(),
		newServiceDeautomate(svcopts).Command(),
		newServiceLock(svcopts).Command(),
		newServiceUnlock(svcopts).Command(),
		newGetConfig(opts).Command(),
		newSetConfig(opts).Command(),
	)

	return cmd
}

func (opts *rootOpts) PersistentPreRunE(cmd *cobra.Command, _ []string) error {
	opts.URL = getFromEnvIfNotSet(cmd.Flags(), "url", envVariableURL, opts.URL)
	if _, err := url.Parse(opts.URL); err != nil {
		return errors.Wrapf(err, "parsing URL")
	}
	opts.FluxSVC = transport.NewClient(http.DefaultClient, transport.NewRouter(), opts.URL, flux.Token(opts.Token))
	return nil
}

func getFromEnvIfNotSet(flags *pflag.FlagSet, flagName, envName, value string) string {
	if flags.Changed(flagName) {
		return value
	}
	if env := os.Getenv(envName); env != "" {
		return env
	}
	return value // not changed, so presumably the default
}
