package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type serviceDeautomateOpts struct {
	*rootOpts
	service string
	outputOpts
	cause update.Cause
}

func newServiceDeautomate(parent *rootOpts) *serviceDeautomateOpts {
	return &serviceDeautomateOpts{rootOpts: parent}
}

func (opts *serviceDeautomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deautomate",
		Short: "Turn off automatic deployment for a service.",
		Example: makeExample(
			"fluxctl deautomate --service=helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to deautomate")
	return cmd
}

func (opts *serviceDeautomateOpts) RunE(cmd *cobra.Command, args []string) error {
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
		serviceID: policy.Update{Remove: policy.Set{policy.Automated: "true"}},
	}, opts.cause)
	if err != nil {
		return err
	}
	return await(cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, false, opts.verbose)
}
