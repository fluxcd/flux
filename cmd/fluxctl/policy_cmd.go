package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type controllerPolicyOpts struct {
	*rootOpts
	outputOpts

	namespace  string
	controller string
	tagAll     string
	tags       []string

	automate, deautomate bool
	lock, unlock         bool

	cause update.Cause

	// Deprecated
	service string
}

func newControllerPolicy(parent *rootOpts) *controllerPolicyOpts {
	return &controllerPolicyOpts{rootOpts: parent}
}

func (opts *controllerPolicyOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policies for a controller.",
		Long: `
Manage policies for a controller.

Tag filter patterns must be specified as 'container=pattern', such as 'foo=1.*'
where an asterisk means 'match anything'.
Surrounding these with single-quotes are recommended to avoid shell expansion.

If both --tag-all and --tag are specified, --tag-all will apply to all
containers which aren't explicitly named.
        `,
		Example: makeExample(
			"fluxctl policy --controller=deployment/foo --automate",
			"fluxctl policy --controller=deployment/foo --lock",
			"fluxctl policy --controller=deployment/foo --tag='bar=1.*' --tag='baz=2.*'",
			"fluxctl policy --controller=deployment/foo --tag-all='master-*' --tag='bar=1.*'",
		),
		RunE: opts.RunE,
	}

	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	flags := cmd.Flags()
	flags.StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	flags.StringVarP(&opts.controller, "controller", "c", "", "Controller to modify")
	flags.StringVar(&opts.tagAll, "tag-all", "", "Tag filter pattern to apply to all containers")
	flags.StringSliceVar(&opts.tags, "tag", nil, "Tag filter container/pattern pairs")
	flags.BoolVar(&opts.automate, "automate", false, "Automate controller")
	flags.BoolVar(&opts.deautomate, "deautomate", false, "Deautomate controller")
	flags.BoolVar(&opts.lock, "lock", false, "Lock controller")
	flags.BoolVar(&opts.unlock, "unlock", false, "Unlock controller")

	// Deprecated
	flags.StringVarP(&opts.service, "service", "s", "", "Service to modify")
	flags.MarkHidden("service")

	return cmd
}

func (opts *controllerPolicyOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}
	if len(args) > 0 {
		return errorWantedNoArgs
	}
	if opts.controller == "" {
		return newUsageError("-c, --controller is required")
	}
	if opts.automate && opts.deautomate {
		return newUsageError("automate and deautomate both specified")
	}
	if opts.lock && opts.unlock {
		return newUsageError("lock and unlock both specified")
	}

	resourceID, err := flux.ParseResourceIDOptionalNamespace(opts.namespace, opts.controller)
	if err != nil {
		return err
	}

	changes, err := calculatePolicyChanges(opts)
	if err != nil {
		return err
	}

	ctx := context.Background()
	updates := policy.Updates{
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
	return await(ctx, cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, false, opts.verbosity)
}

func calculatePolicyChanges(opts *controllerPolicyOpts) (policy.Update, error) {
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
		add = add.Set(policy.TagAll, "glob:"+opts.tagAll)
	}

	for _, tagPair := range opts.tags {
		parts := strings.Split(tagPair, "=")
		if len(parts) != 2 {
			return policy.Update{}, fmt.Errorf("invalid container/tag pair: %q. Expected format is 'container=filter'", tagPair)
		}

		container, tag := parts[0], parts[1]
		if tag != "*" {
			add = add.Set(policy.TagPrefix(container), "glob:"+tag)
		} else {
			remove = remove.Add(policy.TagPrefix(container))
		}
	}

	return policy.Update{
		Add:    add,
		Remove: remove,
	}, nil
}
