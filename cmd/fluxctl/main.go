package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/platform"
)

func main() {
	rootOpts := &rootOpts{}
	rootCmd := &cobra.Command{
		Use:          "fluxctl",
		Short:        "fluxctl is a commandline client for the fluxd daemon.",
		SilenceUsage: true,
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
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type rootOpts struct {
	URL string
}

type serviceOpts struct {
	*rootOpts
	Namespace string
}

type serviceListOpts struct {
	*serviceOpts
}

func (opts *serviceListOpts) RunE(*cobra.Command, []string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf(
		"%s/services?namespace=%s",
		opts.URL,
		url.QueryEscape(opts.Namespace),
	), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Status)
		io.Copy(os.Stderr, resp.Body)
		return nil
	}

	var services []platform.Service
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return nil
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

	req, err := http.NewRequest("POST", fmt.Sprintf(
		"%s/release?namespace=%s&service=%s&updatePeriod=%s",
		opts.URL,
		url.QueryEscape(opts.Namespace),
		url.QueryEscape(opts.Service),
		url.QueryEscape(opts.UpdatePeriod.String()),
	), bytes.NewReader(buf))
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.Service, opts.UpdatePeriod.String())
	begin := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	took := time.Since(begin).String()
	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Fprintf(os.Stdout, "success! (%s)\n", took)
	default:
		fmt.Fprintf(os.Stdout, "failed! %s (%s)\n", resp.Status, took)
		io.Copy(os.Stdout, resp.Body)
	}

	return nil
}
