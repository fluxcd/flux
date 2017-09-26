package main

import (
	"github.com/spf13/cobra"

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
	policyOpts := &servicePolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		service:    opts.service,
		cause:      opts.cause,
		deautomate: true,
	}
	return policyOpts.RunE(policyOpts.Command(), args)
}
