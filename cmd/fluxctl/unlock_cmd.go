package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceUnlockOpts struct {
	*serviceOpts
	service string
}

func newServiceUnlock(parent *serviceOpts) *serviceUnlockOpts {
	return &serviceUnlockOpts{serviceOpts: parent}
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
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to unlock")
	return cmd
}

func (opts *serviceUnlockOpts) RunE(_ *cobra.Command, args []string) error {
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

	return opts.FluxSVC.Unlock(noInstanceID, serviceID)
}
