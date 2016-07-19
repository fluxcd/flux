package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type rootOpts struct {
	URL   string
	Fluxd flux.Service
}

func newRoot() *rootOpts {
	return &rootOpts{}
}

func (opts *rootOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "fluxctl",
		Short:             "fluxctl helps you deploy your code.",
		SilenceUsage:      true,
		PersistentPreRunE: opts.PersistentPreRunE,
	}
	cmd.PersistentFlags().StringVarP(&opts.URL, "url", "u", "http://localhost:3030/v0", "base URL of the fluxd API server")
	return cmd
}

func (opts *rootOpts) PersistentPreRunE(*cobra.Command, []string) error {
	var err error
	opts.Fluxd, err = flux.NewClient(opts.URL)
	return err
}
