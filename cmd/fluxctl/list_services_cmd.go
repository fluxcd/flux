package main

import (
	"errors"

	"github.com/spf13/cobra"
)

type serviceListOpts struct {
	*rootOpts
	namespace string
}

func newServiceList(parent *rootOpts) *serviceListOpts {
	return &serviceListOpts{rootOpts: parent}
}

func (opts *serviceListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "list-services",
		Short:  "Deprecated - use list-workloads instead",
		Hidden: true,
		RunE:   opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace to query, blank for all namespaces")
	return cmd
}

func (opts *serviceListOpts) RunE(cmd *cobra.Command, args []string) error {
	return errors.New("list-services is deprecated, use list-workloads instead")
}
