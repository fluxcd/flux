package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	v6 "github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/policy"
)

type workloadListOpts struct {
	*rootOpts
	namespace     string
	allNamespaces bool
	containerName string
}

func newWorkloadList(parent *rootOpts) *workloadListOpts {
	return &workloadListOpts{rootOpts: parent}
}

func (opts *workloadListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-workloads",
		Aliases: []string{"list-controllers"}, // Transient backwards compatibility after replacing controller by workload
		Short:   "List workloads currently running in the cluster.",
		Example: makeExample("fluxctl list-workloads"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Confine query to namespace")
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "a", false, "Query across all namespaces")
	cmd.Flags().StringVarP(&opts.containerName, "container", "c", "", "Filter workloads by container name")
	return cmd
}

func (opts *workloadListOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	var ns string
	if opts.allNamespaces {
		ns = ""
	} else {
		ns = getKubeConfigContextNamespaceOrDefault(opts.namespace, "default", opts.Context)
	}

	ctx := context.Background()

	workloads, err := opts.API.ListServices(ctx, ns)
	if err != nil {
		return err
	}

	if opts.containerName != "" {
		workloads = filterByContainerName(workloads, opts.containerName)
	}

	sort.Sort(workloadStatusByName(workloads))

	w := newTabwriter()
	fmt.Fprintf(w, "WORKLOAD\tCONTAINER\tIMAGE\tRELEASE\tPOLICY\n")
	for _, workload := range workloads {
		if len(workload.Containers) > 0 {
			c := workload.Containers[0]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", workload.ID, c.Name, c.Current.ID, workload.Status, policies(workload))
			for _, c := range workload.Containers[1:] {
				fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
			}
		} else {
			fmt.Fprintf(w, "%s\t\t\t%s\t%s\n", workload.ID, workload.Status, policies(workload))
		}
	}
	w.Flush()
	return nil
}

type workloadStatusByName []v6.ControllerStatus

func (s workloadStatusByName) Len() int {
	return len(s)
}

func (s workloadStatusByName) Less(a, b int) bool {
	return s[a].ID.String() < s[b].ID.String()
}

func (s workloadStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

func policies(s v6.ControllerStatus) string {
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

// Extract workloads having its container name equal to containerName
func filterByContainerName(workloads []v6.ControllerStatus, containerName string) (filteredWorkloads []v6.ControllerStatus) {
    for _, workload := range workloads {
        if len(workload.Containers) > 0 {
			for _, c := range workload.Containers {
				if c.Name == containerName {
					filteredWorkloads = append(filteredWorkloads, workload)
					break
				}
			}
		}
    }
    return
}
