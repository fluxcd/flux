package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/git"
)

type setupOpts struct {
	gitURL             string
	gitBranch          string
	gitFluxPath        string
	gitLabel           string
	timeout            time.Duration
	namespace          string
	additionalFluxArgs []string
	amend              bool
}

func newSetup() *setupOpts {
	return &setupOpts{}
}

func (opts *setupOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Bootstrap Flux, installing it in the cluster and initializing its manifests in the Git repository",
		RunE:  opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.gitURL, "git-url", "r", "",
		"URL of the Git repository to be used by Flux, e.g. git@github.com:<your username>/flux-get-started")
	cmd.Flags().StringVarP(&opts.gitBranch, "git-branch", "b", "master",
		"Git branch to be used by Flux")
	cmd.Flags().StringVarP(&opts.gitLabel, "git-label", "l", "flux",
		"Git label to keep track of FLux's sync progress; overrides both --git-sync-tag and --git-notes-ref")
	cmd.Flags().StringVarP(&opts.gitFluxPath, "git-flux-subdir", "p", "flux/",
		"Directory within the Git repository where to commit the Flux manifests")
	cmd.Flags().DurationVarP(&opts.timeout, "timeout", "t", 10*time.Second,
		"Timeout duration for I/O operations")
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "flux",
		"Cluster namespace where to install flux")
	cmd.Flags().BoolVarP(&opts.amend, "amend", "a", false,
		"Stop for amending the Flux manifests before pushing them to the Git repository")
	cmd.Flags().StringSliceVarP(&opts.additionalFluxArgs, "extra-flux-args", "e", []string{},
		"Additional arguments for Flux as CSVs, e.g. --extra-flux-arg='--manifest-generation=true,--sync-garbage-collection=true'")
	return cmd
}

func (opts *setupOpts) RunE(cmd *cobra.Command, args []string) error {
	//0. Read and patch embedded deploy/* manifests with what was passed to ops

	// TODO. BTW, it's probably easier to embed chart/flux/* and use `helm template` as a library instead

	//1. Clone repository. In the future we could optionally create the repository automatically for the most
	//   popular git providers (e.g. GitHub OAuth).

	if opts.gitURL == "" {
		fmt.Errorf("no Git repository was provided, please provide a repository through --git-url")
	}

	remote := git.Remote{opts.gitURL}
	pollInterval := 0 * time.Second // there is no need for polling, We will fetch the repo once, manually
	repo := git.NewRepo(remote, git.Branch(opts.gitBranch), git.Timeout(opts.timeout), git.PollInterval(pollInterval))
	checkoutConfig := git.Config{
		Branch: opts.gitBranch,
	}
	ctx := context.Background()
	cloneCtx, cloneCtxCancel := context.WithTimeout(ctx, opts.timeout)
	defer cloneCtxCancel()
	checkout, err := repo.Clone(cloneCtx, checkoutConfig)
	if err != nil {
		return fmt.Errorf("cannot clone repository %s: %s", opts.gitURL)
	}
	cleanCheckout := true
	defer func() {
		if cleanCheckout {
			checkout.Clean()
		}
	}()

	// 2. Write manifests from (0) to the repository's clone working directory

	// 3. If --amend was passed, stop to let the user edit the manifests, by prompting a shell

	// 4. `kubectl apply` the changes to the cluster

	// 5. commit the local changes and push to the remote repository

	// 6. Make sure that the ssh key is added to the repo. At this point we should probably just print the ssh
	//    key and telling the user to add it manually. In the future we can do this programatically for the most popular
	//    Git providers.

	// 7. Confirm that Flux is running (for instance by checking that its own manifests have been synced, this can be
	//    done by checking that the sync annotations have been added)

	return nil
}
