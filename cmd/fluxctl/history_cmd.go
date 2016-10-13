package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type serviceHistoryOpts struct {
	*serviceOpts
	service string
}

func newServiceHistory(parent *serviceOpts) *serviceHistoryOpts {
	return &serviceHistoryOpts{serviceOpts: parent}
}

func (opts *serviceHistoryOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show the history of a service or all services",
		Example: makeExample(
			"fluxctl history --service=default/foo",
			"fluxctl history",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service for which to show history; if left empty, history for all services is shown")
	return cmd
}

func (opts *serviceHistoryOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	service, err := parseServiceOption(opts.service)
	if err != nil {
		return err
	}

	events, err := opts.FluxSVC.History(noInstanceID, service)
	if err != nil {
		return err
	}

	out := newTabwriter()

	fmt.Fprintln(out, "TIME\tTYPE\tMESSAGE")
	for _, event := range events {
		fmt.Fprintf(out, "%s\t%s\t%s\n", event.Stamp.Format(time.RFC822), event.Type, event.Data)
	}

	out.Flush()
	return nil
}
