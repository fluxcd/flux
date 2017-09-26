package main

import (
	"github.com/spf13/cobra"

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
	policyOpts := &servicePolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		service:    opts.service,
		cause:      opts.cause,
		lock:       true,
	}
	return policyOpts.RunE(policyOpts.Command(), args)
}
