package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/history"
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
			"fluxctl service history --service=helloworld",
			"fluxctl service history",
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
	histories, err := opts.Fluxd.History(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	out := newTabwriter()

	if opts.service != "" {
		fmt.Fprintln(out, "TIME\tMESSAGE")
		if history, found := histories[opts.service]; found {
			for _, event := range history.Events {
				fmt.Fprintf(out, "%s\t%s\n", event.Stamp, event.Msg)
			}
		}
	} else {
		events := make([]serviceEvent, 0, 0)
		for _, h := range histories {
			for _, e := range h.Events {
				events = append(events, serviceEvent{e, h.Service})
			}
		}
		sort.Sort(serviceEventLog(events))
		fmt.Fprintln(out, "TIME\tSERVICE\tMESSAGE")
		for _, e := range events {
			fmt.Fprintf(out, "%s\t%s\t%s\n", e.Stamp, e.service, e.Msg)
		}
	}

	out.Flush()
	return nil
}

type serviceEvent struct {
	history.Event
	service string
}

type serviceEventLog []serviceEvent

// The natural sort order is descending order of timestamp.
func (e serviceEventLog) Len() int           { return len(e) }
func (e serviceEventLog) Less(i, j int) bool { return e[i].Stamp.After(e[j].Stamp) }
func (e serviceEventLog) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
