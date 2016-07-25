package main

import (
	"github.com/spf13/cobra"
)

type repoOpts struct {
	*rootOpts
	repository string
}

func newRepo(parent *rootOpts) *repoOpts {
	return &repoOpts{rootOpts: parent}
}

func (opts *repoOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repository",
		Aliases: []string{"repo"},
		Short:   "Subcommands dealing with image repositories, e.g., quay.io/weaveworks/helloworld",
	}
	cmd.PersistentFlags().StringVarP(&opts.repository, "repo", "r", "", "The repository in question, e.g., quay.io/weaveworks/helloworld (required)")
	return cmd
}
