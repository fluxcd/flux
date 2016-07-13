package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootOpts := &rootOpts{}
	rootCmd := &cobra.Command{
		Use:          "fluxctl",
		Short:        "fluxctl is a commandline client for the fluxd daemon.",
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVarP(&rootOpts.URL, "url", "u", "http://localhost:3030/v0/", "base URL of the fluxd API server")

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

	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceListCmd)

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
		opts.Namespace,
	), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()
	return nil
}
