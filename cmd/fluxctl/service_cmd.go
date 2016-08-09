package main

import (
	"github.com/spf13/cobra"
)

type serviceOpts struct {
	*rootOpts
	namespace string
}

func newService(parent *rootOpts) *serviceOpts {
	return &serviceOpts{rootOpts: parent}
}

func (opts *serviceOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service <list, ...> [options]",
		Short: "Manipulate platform services.",
	}
	cmd.PersistentFlags().StringVarP(&opts.namespace, "namespace", "n", "default", "namespace to introspect")

	cmd.AddCommand(
		newServiceList(opts).Command(),
		newServiceShow(opts).Command(),
		newServiceRelease(opts).Command(),
		newServiceHistory(opts).Command(),
		newServiceAutomate(opts).Command(),   // TODO(pb): temporarily disabled
		newServiceDeautomate(opts).Command(), // TODO(pb): temporarily disabled
	)

	return cmd
}
