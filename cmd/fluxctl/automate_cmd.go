package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type serviceAutomateOpts struct {
	*rootOpts
	service string
	outputOpts
	cause update.Cause
}

func newServiceAutomate(parent *rootOpts) *serviceAutomateOpts {
	return &serviceAutomateOpts{rootOpts: parent}
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
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to automate")
	return cmd
}

func (opts *serviceAutomateOpts) RunE(cmd *cobra.Command, args []string) error {
	policyOpts := &servicePolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		service:    opts.service,
		cause:      opts.cause,
		automate:   true,
	}
	return policyOpts.RunE(cmd, args)
}
