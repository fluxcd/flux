package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	rootOpts := &rootOpts{}
	rootCmd := &cobra.Command{
		Use:          "fluxctl",
		Short:        "fluxctl is a commandline client for the fluxd daemon.",
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVarP(&rootOpts.URL, "url", "u", "http://localhost:3030/v0", "base URL of the fluxd API server")

	serviceCmd := &cobra.Command{
		Use:   "service <list, ...> [options]",
		Short: "Manipulate platform services.",
	}

	serviceListOpts := &serviceListOpts{rootOpts: rootOpts}
	serviceListCmd := &cobra.Command{
		Use:   "list",
		Short: "List services currently running on the platform.",
		RunE:  serviceListOpts.RunE,
	}
	serviceListCmd.Flags().StringVarP(&serviceListOpts.Namespace, "namespace", "n", "default", "namespace to introspect")

	serviceReleaseOpts := &serviceReleaseOpts{rootOpts: rootOpts}
	serviceReleaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		RunE:  serviceReleaseOpts.RunE,
	}
	serviceReleaseCmd.Flags().StringVarP(&serviceReleaseOpts.Namespace, "namespace", "n", "default", "namespace to introspect")
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

type serviceListOpts struct {
	*rootOpts
	Namespace string
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

	io.Copy(os.Stdout, resp.Body)
	return nil
}

type serviceReleaseOpts struct {
	*rootOpts
	Namespace    string
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

	fmt.Fprintf(os.Stderr, "%s\n", req.URL.String())
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
