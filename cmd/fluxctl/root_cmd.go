package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/justinbarrick/go-k8s-portforward"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/weaveworks/flux/api"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rootOpts struct {
	URL       string
	Token     string
	Namespace string
	API       api.Server
}

func newRoot() *rootOpts {
	return &rootOpts{}
}

var rootLongHelp = strings.TrimSpace(`
fluxctl helps you deploy your code.

Connecting:

  # To a fluxd running in namespace "default" in your current kubectl context
  fluxctl list-controllers

  # To a fluxd running in namespace "weave" in your current kubectl context
  fluxctl --k8s-fwd-ns=weave list-controllers

  # To a Weave Cloud instance, with your instance token in $TOKEN
  fluxctl --token $TOKEN list-controllers

Workflow:
  fluxctl list-controllers                                                   # Which controllers are running?
  fluxctl list-images --controller=default:deployment/foo                    # Which images are running/available?
  fluxctl release --controller=default:deployment/foo --update-image=bar:v2  # Release new version.
`)

const (
	envVariableURL        = "FLUX_URL"
	envVariableNamespace  = "FLUX_FORWARD_NAMESPACE"
	envVariableToken      = "FLUX_SERVICE_TOKEN"
	envVariableCloudToken = "WEAVE_CLOUD_TOKEN"
	defaultURLGivenToken  = "https://cloud.weave.works/api/flux"
)

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Long:              rootLongHelp,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}

	cmd.PersistentFlags().StringVar(&opts.Namespace, "k8s-fwd-ns", "default",
		fmt.Sprintf("Namespace in which fluxd is running, for creating a port forward to access the API. No port forward will be created if a URL or token is given. You can also set the environment variable %s", envVariableNamespace))
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "",
		fmt.Sprintf("Base URL of the flux API (defaults to %q if a token is provided); you can also set the environment variable %s", defaultURLGivenToken, envVariableURL))
	cmd.PersistentFlags().StringVarP(&opts.Token, "token", "t", "",
		fmt.Sprintf("Weave Cloud authentication token; you can also set the environment variable %s or %s", envVariableCloudToken, envVariableToken))

	cmd.AddCommand(
		newVersionCommand(),
		newServiceList(opts).Command(),
		newControllerShow(opts).Command(),
		newControllerList(opts).Command(),
		newControllerRelease(opts).Command(),
		newServiceAutomate(opts).Command(),
		newControllerDeautomate(opts).Command(),
		newControllerLock(opts).Command(),
		newControllerUnlock(opts).Command(),
		newControllerPolicy(opts).Command(),
		newSave(opts).Command(),
		newIdentity(opts).Command(),
		newSync(opts).Command(),
	)

	return cmd
}

func (opts *rootOpts) PersistentPreRunE(cmd *cobra.Command, _ []string) error {
	opts.Namespace = getFromEnvIfNotSet(cmd.Flags(), "k8s-fwd-ns", opts.Namespace, envVariableNamespace)
	opts.Token = getFromEnvIfNotSet(cmd.Flags(), "token", opts.Token, envVariableToken, envVariableCloudToken)
	opts.URL = getFromEnvIfNotSet(cmd.Flags(), "url", opts.URL, envVariableURL)

	if opts.Token != "" && opts.URL == "" {
		opts.URL = defaultURLGivenToken
	}

	if opts.URL == "" {
		portforwarder, err := portforward.NewPortForwarder(opts.Namespace, metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "name",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"flux", "fluxd", "weave-flux-agent"},
				},
			},
		}, 3030)
		if err != nil {
			return errors.Wrap(err, "initializing port forwarder")
		}

		err = portforwarder.Start()
		if err != nil {
			return errors.Wrap(err, "creating port forward")
		}

		opts.URL = fmt.Sprintf("http://127.0.0.1:%d/api/flux", portforwarder.ListenPort)
	}

	if _, err := url.Parse(opts.URL); err != nil {
		return errors.Wrapf(err, "parsing URL")
	}

	opts.API = client.New(http.DefaultClient, transport.NewAPIRouter(), opts.URL, client.Token(opts.Token))
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
