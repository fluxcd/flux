package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceListOpts struct {
	*serviceOpts
}

func newServiceList(parent *serviceOpts) *serviceListOpts {
	return &serviceListOpts{serviceOpts: parent}
}

func (opts *serviceListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-services",
		Short:   "List services currently running on the platform.",
		Example: makeExample("fluxctl list-services --namespace=default"),
		RunE:    opts.RunE,
	}
	return cmd
}

func (opts *serviceListOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	services, err := opts.Fluxd.ListServices()
	if err != nil {
		return err
	}

	w := newTabwriter()
	fmt.Fprintf(w, "SERVICE\tCONTAINER\tIMAGE\tRELEASE\n")
	for _, s := range services {
		c := s.Containers[0]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, c.Name, maybeUpToDate(c.Current, c.Available), s.Status, maybeAutomated(s))
		for _, c := range s.Containers[1:] {
			fmt.Fprintf(w, "\t%s\t%s\n", c.Name, maybeUpToDate(c.Current, c.Available))
		}
	}
	w.Flush()
	return nil
}

func maybeUpToDate(current flux.ImageDescription, available []flux.ImageDescription) string {
	if len(available) > 0 && current.ID == available[0].ID {
		return "* " + string(current.ID)
	}
	return string(current.ID)
}

func maybeAutomated(s flux.ServiceStatus) string {
	if s.Automated {
		return "automated"
	}
	return ""
}
