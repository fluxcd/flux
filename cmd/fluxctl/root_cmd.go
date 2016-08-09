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

func newRoot() *rootOpts {
	return &rootOpts{}
}

var rootLongHelp = strings.TrimSpace(`
fluxctl helps you deploy your code.

Workflow:
  fluxctl service list                                                                 # Which services are running?
  fluxctl service show -s helloworld                                                   # Which images are running/available?
  fluxctl config update -f rc.yaml -i quay.io/weaveworks/helloworld:de9f3b2 -o rc.yaml # Update file to use new image.
  fluxctl service release -s helloworld -f rc.yaml                                     # Release new version.
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

	cmd.AddCommand(
		newService(opts).Command(),
		newImage(opts).Command(),
		newConfig(opts).Command(),
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
