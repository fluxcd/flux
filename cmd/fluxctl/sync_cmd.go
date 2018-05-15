package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/git"
	"github.com/weaveworks/flux/update"
)

type syncOpts struct {
	*rootOpts
}

func newSync(parent *rootOpts) *syncOpts {
	return &syncOpts{rootOpts: parent}
}

func (opts *syncOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "synchronize the cluster with the git repository, now",
		RunE:  opts.RunE,
	}
	return cmd
}

func (opts *syncOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	ctx := context.Background()

	gitConfig, err := opts.API.GitRepoConfig(ctx, false)
	if err != nil {
		return err
	}

	switch gitConfig.Status {
	case git.RepoNoConfig:
		return fmt.Errorf("no git repository is configured")
	case git.RepoReady:
		break
	default:
		return fmt.Errorf("git repository %s is not ready to sync (status: %s)", gitConfig.Remote.URL, string(gitConfig.Status))
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Synchronizing with %s\n", gitConfig.Remote.URL)

	updateSpec := update.Spec{
		Type: update.Sync,
		Spec: update.ManualSync{},
	}
	jobID, err := opts.API.UpdateManifests(ctx, updateSpec)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStderr(), "Job ID %s\n", string(jobID))
	result, err := awaitJob(ctx, opts.API, jobID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStderr(), "HEAD of %s is %s\n", gitConfig.Remote.Branch, result.Revision)
	err = awaitSync(ctx, opts.API, result.Revision)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStderr(), "Applied %s\n", result.Revision)
	fmt.Fprintln(cmd.OutOrStderr(), "Done.")
	return nil
}
