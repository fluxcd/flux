package main

import (
	"github.com/spf13/cobra"
)

type imageOpts struct {
	*rootOpts
	repository string
}

func newImage(parent *rootOpts) *imageOpts {
	return &imageOpts{rootOpts: parent}
}

func (opts *imageOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Subcommands dealing with image repositories, e.g., quay.io/weaveworks/helloworld",
	}
	cmd.PersistentFlags().StringVarP(&opts.repository, "repo", "r", "", "The repository in question, e.g., quay.io/weaveworks/helloworld (required)")

	cmd.AddCommand(newImageList(opts).Command())

	return cmd
}
