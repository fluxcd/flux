package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type serviceStatusOpts struct {
	*serviceOpts
}

func newServiceStatus(parent *serviceOpts) *serviceStatusOpts {
	return &serviceStatusOpts{serviceOpts: parent}
}

func (opts *serviceStatusOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the release status of each service running on the platform",
		RunE:  opts.RunE,
	}
	return cmd
}

func (opts *serviceStatusOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	status, err := opts.Fluxd.ReleasesStatus(opts.namespace)
	if err != nil {
		return err
	}

	w := newTabwriter()
	fmt.Fprintln(w, "SERVICE\tSTATUS")
	for _, s := range status {
		fmt.Fprintf(w, "%s\t%s\n", s.Service.Name, s.Status)
	}
	w.Flush()
	return nil
}
