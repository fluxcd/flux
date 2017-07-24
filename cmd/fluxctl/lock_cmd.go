package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type serviceLockOpts struct {
	*rootOpts
	service string
	outputOpts
	cause update.Cause
}

func newServiceLock(parent *rootOpts) *serviceLockOpts {
	return &serviceLockOpts{rootOpts: parent}
}

func (opts *serviceLockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock a service, so it cannot be deployed.",
		Example: makeExample(
			"fluxctl lock --service=helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to lock")
	return cmd
}

func (opts *serviceLockOpts) RunE(cmd *cobra.Command, args []string) error {
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
		serviceID: policy.Update{Add: policy.Set{policy.Locked: "true"}},
	}, opts.cause)
	if err != nil {
		return err
	}
	return await(cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, false, opts.verbose)
}
