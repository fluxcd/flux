package main

import "github.com/spf13/cobra"

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

func (opts *serviceAutomateOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}
	return opts.Fluxd.Automate(opts.namespace, opts.service)
}
