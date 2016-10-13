package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceListOpts struct {
	*serviceOpts
	namespace string
}

func newServiceList(parent *serviceOpts) *serviceListOpts {
	return &serviceListOpts{serviceOpts: parent}
}

func (opts *serviceListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-services",
		Short:   "List services currently running on the platform.",
		Example: makeExample("fluxctl list-services"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace to query, blank for all namespaces")
	return cmd
}

func (opts *serviceListOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	services, err := opts.FluxSVC.ListServices(noInstanceID, opts.namespace)
	if err != nil {
		return err
	}

	sort.Sort(serviceStatusByName(services))

	w := newTabwriter()
	fmt.Fprintf(w, "SERVICE\tCONTAINER\tIMAGE\tRELEASE\tPOLICY\n")
	for _, s := range services {
		if len(s.Containers) > 0 {
			c := s.Containers[0]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, c.Name, c.Current.ID, s.Status, s.Policies())
			for _, c := range s.Containers[1:] {
				fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
			}
		} else {
			fmt.Fprintf(w, "%s\t\t\t\t\n", s.ID)
		}
	}
	w.Flush()
	return nil
}

type serviceStatusByName []flux.ServiceStatus

func (s serviceStatusByName) Len() int {
	return len(s)
}

func (s serviceStatusByName) Less(a, b int) bool {
	return s[a].ID < s[b].ID
}

func (s serviceStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}
