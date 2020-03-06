package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/fluxcd/flux/pkg/api"
	transport "github.com/fluxcd/flux/pkg/http"
	"github.com/fluxcd/flux/pkg/http/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rootOpts struct {
	Context   string
	URL       string
	Token     string
	Namespace string
	Labels    map[string]string
	API       api.Server
	Timeout   time.Duration
}

func newRoot() *rootOpts {
	return &rootOpts{}
}

var rootLongHelp = strings.TrimSpace(`
fluxctl helps you deploy your code.

Connecting:

  # To a fluxd running in namespace "default" in your current kubectl context
  fluxctl list-workloads

  # To a fluxd running in namespace "weave" in your current kubectl context
  fluxctl --k8s-fwd-ns=weave list-workloads

  # To a Weave Cloud instance, with your instance token in $TOKEN
  fluxctl --token $TOKEN list-workloads

Workflow:
  fluxctl list-workloads                                                   # Which workloads are running?
  fluxctl list-images --workload=default:deployment/foo                    # Which images are running/available?
  fluxctl release --workload=default:deployment/foo --update-image=bar:v2  # Release new version.
`)

const (
	defaultURLGivenToken  = "https://cloud.weave.works/api/flux"
	envVariableURL        = "FLUX_URL"
	envVariableNamespace  = "FLUX_FORWARD_NAMESPACE"
	envVariableLabels     = "FLUX_FORWARD_LABELS"
	envVariableToken      = "FLUX_SERVICE_TOKEN"
	envVariableCloudToken = "WEAVE_CLOUD_TOKEN"
	envVariableTimeout    = "FLUX_TIMEOUT"
)

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Long:              rootLongHelp,
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}

	cmd.PersistentFlags().StringVar(&opts.Context, "context", "",
		fmt.Sprint("The kubeconfig context to use, will use your current selected if not specified"))
	cmd.PersistentFlags().StringVar(&opts.Namespace, "k8s-fwd-ns", "default",
		fmt.Sprintf("Namespace in which fluxd is running, for creating a port forward to access the API. No port forward will be created if a URL or token is given. You can also set the environment variable %s", envVariableNamespace))
	cmd.PersistentFlags().StringToStringVar(&opts.Labels, "k8s-fwd-labels", map[string]string{"app": "flux"},
		fmt.Sprintf("Labels used to select the fluxd pod a port forward should be created for. You can also set the environment variable %s", envVariableLabels))
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "",
		fmt.Sprintf("Base URL of the Flux API (defaults to %q if a token is provided); you can also set the environment variable %s", defaultURLGivenToken, envVariableURL))
	cmd.PersistentFlags().StringVarP(&opts.Token, "token", "t", "",
		fmt.Sprintf("Weave Cloud authentication token; you can also set the environment variable %s or %s", envVariableCloudToken, envVariableToken))
	cmd.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 60*time.Second,
		fmt.Sprintf("Global command timeout; you can also set the environment variable %s", envVariableTimeout))
	cmd.AddCommand(
		newVersionCommand(),
		newImageList(opts).Command(),
		newWorkloadList(opts).Command(),
		newWorkloadRelease(opts).Command(),
		newWorkloadAutomate(opts).Command(),
		newWorkloadDeautomate(opts).Command(),
		newWorkloadLock(opts).Command(),
		newWorkloadUnlock(opts).Command(),
		newWorkloadPolicy(opts).Command(),
		newSave(opts).Command(),
		newIdentity(opts).Command(),
		newSync(opts).Command(),
		newInstall().Command(),
		newCompletionCommand(),
	)

	return cmd
}

func (opts *rootOpts) PersistentPreRunE(cmd *cobra.Command, _ []string) error {
	// skip port forward for certain commands
	switch cmd.Use {
	case "version", "completion SHELL":
		fallthrough
	case "install":
		return nil
	}

	setFromEnvIfNotSet(cmd.Flags(), "k8s-fwd-ns", envVariableNamespace)
	setFromEnvIfNotSet(cmd.Flags(), "k8s-fwd-labels", envVariableLabels)
	setFromEnvIfNotSet(cmd.Flags(), "token", envVariableToken, envVariableCloudToken)
	setFromEnvIfNotSet(cmd.Flags(), "url", envVariableURL)
	setFromEnvIfNotSet(cmd.Flags(), "timeout", envVariableTimeout)

	if opts.Token != "" && opts.URL == "" {
		opts.URL = defaultURLGivenToken
	}

	if opts.URL == "" {
		portforwarder, err := tryPortforwards(opts.Context, opts.Namespace, metav1.LabelSelector{
			MatchLabels: opts.Labels,
		}, metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "name",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"flux", "fluxd", "weave-flux-agent"},
				},
			},
		})
		if err != nil {
			return err
		}

		opts.URL = fmt.Sprintf("http://127.0.0.1:%d/api/flux", portforwarder.ListenPort)
	}

	if _, err := url.Parse(opts.URL); err != nil {
		return errors.Wrapf(err, "parsing URL")
	}

	opts.API = client.New(http.DefaultClient, transport.NewAPIRouter(), opts.URL, client.Token(opts.Token))
	return nil
}

func setFromEnvIfNotSet(flags *pflag.FlagSet, flagName string, envNames ...string) {
	if flags.Changed(flagName) {
		return
	}
	for _, envName := range envNames {
		if env := os.Getenv(envName); env != "" {
			flags.Set(flagName, env)
		}
	}
}
