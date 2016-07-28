package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/history"
)

type serviceListOpts struct {
	*serviceOpts
}

func newServiceList(parent *serviceOpts) *serviceListOpts {
	return &serviceListOpts{serviceOpts: parent}
}

func (opts *serviceListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List services currently running on the platform.",
		Example: makeExample("fluxctl service list --namespace=default"),
		RunE:    opts.RunE,
	}
	return cmd
}

func (opts *serviceListOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	services, err := opts.Fluxd.Services(opts.namespace)
	if err != nil {
		return err
	}
	histories, err := opts.Fluxd.History(opts.namespace, "")
	if err != nil {
		return err
	}

	w := newTabwriter()
	fmt.Fprintf(w, "SERVICE\tIP\tPORTS\tIMAGE\tSTATUS\n")
	for _, s := range services {
		var ports []string
		for _, p := range s.Ports {
			ports = append(ports, fmt.Sprintf("%s/%s→%s", p.External, p.Protocol, p.Internal))
		}
		state := history.StateUnknown
		if status, found := histories[s.Name]; found {
			state = status.State
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.IP, strings.Join(ports, ", "), s.Image, state)
	}
	w.Flush()
	return nil
}
