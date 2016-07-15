package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type serviceListOpts struct {
	*serviceOpts
}

func newServiceList(parent *serviceOpts) *serviceListOpts {
	return &serviceListOpts{serviceOpts: parent}
}

func (opts *serviceListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List services currently running on the platform.",
		RunE:  opts.RunE,
	}
	return cmd
}

func (opts *serviceListOpts) RunE(*cobra.Command, []string) error {
	services, err := opts.Fluxd.Services(opts.Namespace)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tIP\tPORTS\tIMAGE\n")
	for _, s := range services {
		var ports []string
		for _, p := range s.Ports {
			ports = append(ports, fmt.Sprintf("%s/%s→%s", p.External, p.Protocol, p.Internal))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.IP, strings.Join(ports, ", "), s.Image)
	}
	w.Flush()
	return nil
}
