package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type serviceUnlockOpts struct {
	*rootOpts
	service string
	outputOpts
	cause update.Cause
}

func newServiceUnlock(parent *rootOpts) *serviceUnlockOpts {
	return &serviceUnlockOpts{rootOpts: parent}
}

func (opts *serviceUnlockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a service, so it can be deployed.",
		Example: makeExample(
			"fluxctl unlock --service=helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to unlock")
	return cmd
}

func (opts *serviceUnlockOpts) RunE(cmd *cobra.Command, args []string) error {
	policyOpts := &servicePolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		service:    opts.service,
		cause:      opts.cause,
		unlock:     true,
	}
	return policyOpts.RunE(policyOpts.Command(), args)
}
