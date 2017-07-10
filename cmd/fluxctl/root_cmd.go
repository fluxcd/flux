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

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	"github.com/weaveworks/flux/service"
)

type rootOpts struct {
	URL   string
	Token string
	API   api.ClientService
}

// fluxctl never sends an instance ID directly; it's always blank, and
// optionally gets populated by an intermediating authfe from the token.
const noInstanceID = service.InstanceID("")

func newRoot() *rootOpts {
	return &rootOpts{}
}

var rootLongHelp = strings.TrimSpace(`
fluxctl helps you deploy your code.

Workflow:
  fluxctl list-services                                        # Which services are running?
  fluxctl list-images --service=default/foo                    # Which images are running/available?
  fluxctl release --service=default/foo --update-image=bar:v2  # Release new version.
`)

const (
	envVariableURL        = "FLUX_URL"
	envVariableToken      = "FLUX_SERVICE_TOKEN"
	envVariableCloudToken = "WEAVE_CLOUD_TOKEN"
)

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Long:              rootLongHelp,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "https://cloud.weave.works/api/flux",
		fmt.Sprintf("base URL of the flux service; you can also set the environment variable %s", envVariableURL))
	cmd.PersistentFlags().StringVarP(&opts.Token, "token", "t", "",
		fmt.Sprintf("Weave Cloud service token; you can also set the environment variable %s or %s", envVariableCloudToken, envVariableToken))

	cmd.AddCommand(
		newVersionCommand(),
		newServiceShow(opts).Command(),
		newServiceList(opts).Command(),
		newServiceRelease(opts).Command(),
		newServiceAutomate(opts).Command(),
		newServiceDeautomate(opts).Command(),
		newServiceLock(opts).Command(),
		newServiceUnlock(opts).Command(),
		newServicePolicy(opts).Command(),
		newSave(opts).Command(),
		newIdentity(opts).Command(),
	)

	return cmd
}

func (opts *rootOpts) PersistentPreRunE(cmd *cobra.Command, _ []string) error {
	opts.URL = getFromEnvIfNotSet(cmd.Flags(), "url", opts.URL, envVariableURL)
	if _, err := url.Parse(opts.URL); err != nil {
		return errors.Wrapf(err, "parsing URL")
	}
	opts.Token = getFromEnvIfNotSet(cmd.Flags(), "token", opts.Token, envVariableToken, envVariableCloudToken)
	opts.API = client.New(http.DefaultClient, transport.NewAPIRouter(), opts.URL, flux.Token(opts.Token))
	return nil
}

func getFromEnvIfNotSet(flags *pflag.FlagSet, flagName, value string, envNames ...string) string {
	if flags.Changed(flagName) {
		return value
	}
	for _, envName := range envNames {
		if env := os.Getenv(envName); env != "" {
			return env
		}
	}
	return value // not changed, so presumably the default
}
