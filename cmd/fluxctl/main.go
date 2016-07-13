package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

// These are the (unfortunately global) flags.
var (
	OptionURL       string
	OptionNamespace string
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "fluxctl",
		Short:        "fluxctl is a commandline client for the fluxd daemon.",
		SilenceUsage: true,
	}
	rootCmd.Flags().StringVarP(&OptionURL, "url", "u", "http://localhost:3030/v0/", "Base URL of the fluxd API server")

	serviceCmd := &cobra.Command{
		Use:   "service <list, ...> [options]",
		Short: "Manipulate platform services.",
	}

	serviceListCmd := &cobra.Command{
		Use:   "list",
		Short: "List services currently running on the platform.",
		RunE:  serviceList,
	}
	serviceListCmd.Flags().StringVarP(&OptionNamespace, "namespace", "n", "default", "Namespace to introspect")

	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceListCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serviceList(cmd *cobra.Command, args []string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf(
		"%s/services?namespace=%s",
		OptionURL,
		OptionNamespace,
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
