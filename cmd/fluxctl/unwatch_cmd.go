package main

import "github.com/spf13/cobra"

type unwatchOpts struct {
	*rootOpts
}

func newUnwatch(parent *rootOpts) *unwatchOpts {
	return &unwatchOpts{rootOpts: parent}
}

func (opts *unwatchOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unwatch",
		Short: "stop watching for changes in the manifests repo, and modifying the cluster to match.",
		Example: makeExample(
			"fluxctl unwatch",
		),
		RunE: opts.RunE,
	}
	return cmd
}

func (opts *unwatchOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	return opts.API.Unwatch(noInstanceID)
}
