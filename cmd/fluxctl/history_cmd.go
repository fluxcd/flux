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
			"fluxctl history --service=helloworld",
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
	events, err := opts.Fluxd.History(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	out := newTabwriter()

	if opts.service != "" {
		fmt.Fprintln(out, "TIME\tMESSAGE")
		for _, event := range events {
			fmt.Fprintf(out, "%s\t%s\n", event.Stamp.Format(time.RFC822), event.Msg)
		}
	} else {
		fmt.Fprintln(out, "TIME\tSERVICE\tMESSAGE")
		for _, e := range events {
			fmt.Fprintf(out, "%s\t%s\t%s\n", e.Stamp.Format(time.RFC822), e.Service, e.Msg)
		}
	}

	out.Flush()
	return nil
}
