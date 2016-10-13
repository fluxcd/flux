package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceDeautomateOpts struct {
	*serviceOpts
	service string
}

func newServiceDeautomate(parent *serviceOpts) *serviceDeautomateOpts {
	return &serviceDeautomateOpts{serviceOpts: parent}
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
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to deautomate")
	return cmd
}

func (opts *serviceDeautomateOpts) RunE(_ *cobra.Command, args []string) error {
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

	return opts.FluxSVC.Deautomate(noInstanceID, serviceID)
}
