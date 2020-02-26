package main

import (
	"github.com/spf13/cobra"
)

type installOpts struct {
	*baseConfigOpts
}

func newInstall() *installOpts {
	return &installOpts{baseConfigOpts: newBaseConfig()}
}

func (opts *installOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "install",
		Deprecated: "please use base-config",
		Short:      "Print and tweak Kubernetes manifests needed to install Flux in a Cluster",
		RunE:       opts.RunE,
	}
	baseConfigFlags(opts.baseConfigOpts, cmd)

	// Hide and deprecate "git-paths", which was wrongly introduced since its inconsistent with fluxd's git-path flag
	cmd.Flags().MarkHidden("git-paths")
	cmd.Flags().MarkDeprecated("git-paths", "please use --git-path (no ending s) instead")

	return cmd
}

func (opts *installOpts) RunE(cmd *cobra.Command, args []string) error {
	return opts.baseConfigOpts.RunE(cmd, args)
}
