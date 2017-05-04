package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

type serviceAutomateOpts struct {
	*serviceOpts
	service string
}

func newServiceAutomate(parent *serviceOpts) *serviceAutomateOpts {
	return &serviceAutomateOpts{serviceOpts: parent}
}

func (opts *serviceAutomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automate",
		Short: "Turn on automatic deployment for a service.",
		Example: makeExample(
			"fluxctl automate --service=helloworld",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to automate")
	return cmd
}

func (opts *serviceAutomateOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}

	serviceID, err := flux.ParseServiceID(opts.service)
	if err != nil {
		return err
	}

	jobID, err := opts.API.UpdatePolicies(noInstanceID, policy.Updates{
		serviceID: policy.Update{Add: []policy.Policy{policy.Automated}},
	})
	if err != nil {
		return err
	}
	return await(cmd.OutOrStdout(), opts.API, jobID, false)
}
