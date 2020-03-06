package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

type workloadPolicyOpts struct {
	*rootOpts
	outputOpts

	namespace string
	workload  string
	tagAll    string
	tags      []string

	automate, deautomate bool
	lock, unlock         bool

	cause update.Cause

	// Deprecated
	controller string
}

func newWorkloadPolicy(parent *rootOpts) *workloadPolicyOpts {
	return &workloadPolicyOpts{rootOpts: parent}
}

func (opts *workloadPolicyOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policies for a workload.",
		Long: `
Manage policies for a workload.

Tag filter patterns must be specified as 'container=pattern', such as 'foo=1.*'
where an asterisk means 'match anything'.
Surrounding these with single-quotes are recommended to avoid shell expansion.

If both --tag-all and --tag are specified, --tag-all will apply to all
containers which aren't explicitly named.
        `,
		Example: makeExample(
			"fluxctl policy --workload=default:deployment/foo --automate",
			"fluxctl policy --workload=default:deployment/foo --lock",
			"fluxctl policy --workload=default:deployment/foo --tag='bar=1.*' --tag='baz=2.*'",
			"fluxctl policy --workload=default:deployment/foo --tag-all='master-*' --tag='bar=1.*'",
		),
		RunE: opts.RunE,
	}

	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	flags := cmd.Flags()
	flags.StringVarP(&opts.namespace, "namespace", "n", "", "Workload namespace")
	flags.StringVarP(&opts.workload, "workload", "w", "", "Workload to modify")
	flags.StringVar(&opts.tagAll, "tag-all", "", "Tag filter pattern to apply to all containers")
	flags.StringSliceVar(&opts.tags, "tag", nil, "Tag filter container/pattern pairs")
	flags.BoolVar(&opts.automate, "automate", false, "Automate workload")
	flags.BoolVar(&opts.deautomate, "deautomate", false, "Deautomate workload")
	flags.BoolVar(&opts.lock, "lock", false, "Lock workload")
	flags.BoolVar(&opts.unlock, "unlock", false, "Unlock workload")

	// Deprecated
	flags.StringVarP(&opts.controller, "controller", "c", "", "Controller to modify")
	flags.MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadPolicyOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	// Backwards compatibility with --controller until we remove it
	switch {
	case opts.workload == "" && opts.controller == "":
		return newUsageError("-w, --workload is required")
	case opts.workload != "" && opts.controller != "":
		return newUsageError("can't specify both the controller and workload")
	case opts.controller != "":
		opts.workload = opts.controller
	}

	if opts.automate && opts.deautomate {
		return newUsageError("automate and deautomate both specified")
	}
	if opts.lock && opts.unlock {
		return newUsageError("lock and unlock both specified")
	}

	ns := getKubeConfigContextNamespaceOrDefault(opts.namespace, "default", opts.Context)
	resourceID, err := resource.ParseIDOptionalNamespace(ns, opts.workload)
	if err != nil {
		return err
	}

	changes, err := calculatePolicyChanges(opts)
	if err != nil {
		return err
	}

	ctx := context.Background()
	updates := resource.PolicyUpdates{
		resourceID: changes,
	}
	jobID, err := opts.API.UpdateManifests(ctx, update.Spec{
		Type:  update.Policy,
		Cause: opts.cause,
		Spec:  updates,
	})
	if err != nil {
		return err
	}
	return await(ctx, cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, false, opts.verbosity, opts.Timeout)
}

func calculatePolicyChanges(opts *workloadPolicyOpts) (resource.PolicyUpdate, error) {
	add := policy.Set{}
	if opts.automate {
		add = add.Add(policy.Automated)
	}
	if opts.lock {
		add = add.Add(policy.Locked)
		if opts.cause.User != "" {
			add = add.
				Set(policy.LockedUser, opts.cause.User).
				Set(policy.LockedMsg, opts.cause.Message)
		}
	}

	remove := policy.Set{}
	if opts.deautomate {
		remove = remove.Add(policy.Automated)
	}
	if opts.unlock {
		remove = remove.
			Add(policy.Locked).
			Add(policy.LockedMsg).
			Add(policy.LockedUser)
	}
	if opts.tagAll != "" {
		add = add.Set(policy.TagAll, policy.NewPattern(opts.tagAll).String())
	}

	for _, tagPair := range opts.tags {
		parts := strings.Split(tagPair, "=")
		if len(parts) != 2 {
			return resource.PolicyUpdate{}, fmt.Errorf("invalid container/tag pair: %q. Expected format is 'container=filter'", tagPair)
		}

		container, tag := parts[0], parts[1]
		if tag != "*" {
			add = add.Set(policy.TagPrefix(container), policy.NewPattern(tag).String())
		} else {
			remove = remove.Add(policy.TagPrefix(container))
		}
	}

	return resource.PolicyUpdate{
		Add:    add,
		Remove: remove,
	}, nil
}
