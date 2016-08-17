package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/weaveworks/fluxy"
)

const (
	EnvVariableURL = "FLUX_URL"
)

type rootOpts struct {
	URL   string
	Fluxd flux.Service
}

type serviceOpts struct {
	*rootOpts
	namespace string
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
  fluxctl list-services                    # Which services are running?
  fluxctl list-images --service helloworld # Which images are running/available?
  fluxctl release --service helloworld     # Release new version.
  fluxctl history --service helloworld     # Review what happened
`)

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Long:              rootLongHelp,
		SilenceUsage:      true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "http://localhost:3030",
		fmt.Sprintf("base URL of the fluxd API server; you can also set the environment variable %s", EnvVariableURL))

	svcopts := newService(opts)
	cmd.PersistentFlags().StringVarP(&svcopts.namespace, "namespace", "n", "default", "namespace of service(s) to operate on")

	cmd.AddCommand(
		newServiceShow(svcopts).Command(),
		newServiceList(svcopts).Command(),
		newServiceRelease(svcopts).Command(),
		newServiceHistory(svcopts).Command(),
		newServiceAutomate(svcopts).Command(),
		newServiceDeautomate(svcopts).Command(),
	)

	return cmd
}

func (opts *rootOpts) PersistentPreRunE(cmd *cobra.Command, _ []string) error {
	var err error

	opts.URL = getFromEnvIfNotSet(cmd.Flags(), "url", EnvVariableURL, opts.URL)
	opts.Fluxd, err = flux.NewClient(opts.URL)
	return err
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
