package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

func main() {
	rootOpts := &rootOpts{}
	rootCmd := &cobra.Command{
		Use:               "fluxctl",
		Short:             "fluxctl is a commandline client for the fluxd daemon.",
		SilenceUsage:      true,
		PersistentPreRunE: rootOpts.PreRunE,
	}
	rootCmd.PersistentFlags().StringVarP(&rootOpts.URL, "url", "u", "http://localhost:3030/v0", "base URL of the fluxd API server")

	serviceOpts := &serviceOpts{rootOpts: rootOpts}
	serviceCmd := &cobra.Command{
		Use:   "service <list, ...> [options]",
		Short: "Manipulate platform services.",
	}
	serviceCmd.PersistentFlags().StringVarP(&serviceOpts.Namespace, "namespace", "n", "default", "namespace to introspect")

	serviceListOpts := &serviceListOpts{serviceOpts: serviceOpts}
	serviceListCmd := &cobra.Command{
		Use:   "list",
		Short: "List services currently running on the platform.",
		RunE:  serviceListOpts.RunE,
	}

	serviceReleaseOpts := &serviceReleaseOpts{serviceOpts: serviceOpts}
	serviceReleaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		RunE:  serviceReleaseOpts.RunE,
	}
	serviceReleaseCmd.Flags().StringVarP(&serviceReleaseOpts.Service, "service", "s", "", "service to update")
	serviceReleaseCmd.Flags().StringVarP(&serviceReleaseOpts.File, "file", "f", "-", "file containing new ReplicationController definition, or - to read from stdin")
	serviceReleaseCmd.Flags().DurationVarP(&serviceReleaseOpts.UpdatePeriod, "update-period", "p", 5*time.Second, "delay between starting and stopping instances in the rolling update")

	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(imagesCommand(rootOpts))
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type rootOpts struct {
	URL   string
	Fluxd flux.Service
}

func (opts *rootOpts) PreRunE(*cobra.Command, []string) error {
	var err error
	opts.Fluxd, err = flux.NewClient(opts.URL)
	return err
}

type serviceOpts struct {
	*rootOpts
	Namespace string
}

type serviceListOpts struct {
	*serviceOpts
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

type serviceReleaseOpts struct {
	*serviceOpts
	Service      string
	File         string
	UpdatePeriod time.Duration
}

func (opts *serviceReleaseOpts) RunE(*cobra.Command, []string) error {
	if opts.Service == "" {
		return errors.New("-s, --service is required")
	}

	var buf []byte
	var err error
	switch opts.File {
	case "":
		return errors.New("-f, --file is required")

	case "-":
		buf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

	default:
		buf, err = ioutil.ReadFile(opts.File)
		if err != nil {
			return err
		}
	}

	begin := time.Now()
	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.Service, opts.UpdatePeriod.String())
	if err = opts.Fluxd.Release(opts.Namespace, opts.Service, buf, opts.UpdatePeriod); err != nil {
		fmt.Fprintf(os.Stdout, "error! %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
