package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type watchOpts struct {
	*rootOpts
}

func newWatch(parent *rootOpts) *watchOpts {
	return &watchOpts{rootOpts: parent}
}

func (opts *watchOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "begin watching for changes in the manifests repo, and modifying the cluster to match.",
		Example: makeExample(
			"fluxctl watch",
		),
		RunE: opts.RunE,
	}
	return cmd
}

func (opts *watchOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	webhookEndpoint, err := opts.API.Watch(noInstanceID)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Syncing state of manifests repo to the cluster.\n\n")
	fmt.Fprintf(os.Stdout, "To ensure that new changes are applied, create\n")
	fmt.Fprintf(os.Stdout, "a webhook from your repo to:\n%s\n", webhookEndpoint)
	return nil
}
