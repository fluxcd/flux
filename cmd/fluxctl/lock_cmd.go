package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceLockOpts struct {
	*serviceOpts
	service string
}

func newServiceLock(parent *serviceOpts) *serviceLockOpts {
	return &serviceLockOpts{serviceOpts: parent}
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
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to lock")
	return cmd
}

func (opts *serviceLockOpts) RunE(_ *cobra.Command, args []string) error {
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

	return opts.FluxSVC.Lock(noInstanceID, serviceID)
}
