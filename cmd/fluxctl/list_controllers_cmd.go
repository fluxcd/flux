package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
)

type controllerListOpts struct {
	*rootOpts
	namespace     string
	allNamespaces bool
}

func newControllerList(parent *rootOpts) *controllerListOpts {
	return &controllerListOpts{rootOpts: parent}
}

func (opts *controllerListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-controllers",
		Short:   "List controllers currently running in the cluster.",
		Example: makeExample("fluxctl list-controllers"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Confine query to namespace")
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "a", false, "Query across all namespaces")
	return cmd
}

func (opts *controllerListOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if opts.allNamespaces {
		opts.namespace = ""
	}

	ctx := context.Background()

	controllers, err := opts.API.ListServices(ctx, opts.namespace)
	if err != nil {
		return err
	}

	sort.Sort(controllerStatusByName(controllers))

	w := newTabwriter()
	fmt.Fprintf(w, "CONTROLLER\tCONTAINER\tIMAGE\tRELEASE\tPOLICY\n")
	for _, controller := range controllers {
		if len(controller.Containers) > 0 {
			c := controller.Containers[0]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", controller.ID, c.Name, c.Current.ID, controller.Status, policies(controller))
			for _, c := range controller.Containers[1:] {
				fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
			}
		} else {
			fmt.Fprintf(w, "%s\t\t\t\t\n", controller.ID)
		}
	}
	w.Flush()
	return nil
}

type controllerStatusByName []flux.ControllerStatus

func (s controllerStatusByName) Len() int {
	return len(s)
}

func (s controllerStatusByName) Less(a, b int) bool {
	return s[a].ID.String() < s[b].ID.String()
}

func (s controllerStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

func policies(s flux.ControllerStatus) string {
	var ps []string
	if s.Automated {
		ps = append(ps, string(policy.Automated))
	}
	if s.Locked {
		ps = append(ps, string(policy.Locked))
	}
	if s.Ignore {
		ps = append(ps, string(policy.Ignore))
	}
	sort.Strings(ps)
	return strings.Join(ps, ",")
}
