package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/git"
	"github.com/fluxcd/flux/pkg/update"
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
		if gitConfig.Error != "" {
			return fmt.Errorf("git repository %s is not ready to sync\n\nFull error message: %v", gitConfig.Remote.URL, gitConfig.Error)
		}
		return fmt.Errorf("git repository %s is not ready to sync", gitConfig.Remote.URL)
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
	result, err := awaitJob(ctx, opts.API, jobID, opts.Timeout)
	if isUnverifiedHead(err) {
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: %s\n", err)
	} else if err != nil {
		fmt.Fprintf(cmd.OutOrStderr(), "Failed to complete sync job (ID %q)\n", jobID)
		return err
	}

	rev := result.Revision[:7]
	fmt.Fprintf(cmd.OutOrStderr(), "Revision of %s to apply is %s\n", gitConfig.Remote.Branch, rev)
	fmt.Fprintf(cmd.OutOrStderr(), "Waiting for %s to be applied ...\n", rev)
	err = awaitSync(ctx, opts.API, rev, opts.Timeout)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStderr(), "Done.")
	return nil
}

func isUnverifiedHead(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "branch HEAD in the git repo is not verified") &&
			strings.Contains(err.Error(), "last verified commit was"))
}
