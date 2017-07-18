package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type servicePolicyOpts struct {
	*rootOpts
	outputOpts

	service string
	tags    []string

	automate, deautomate bool
	lock, unlock         bool

	cause update.Cause
}

func newServicePolicy(parent *rootOpts) *servicePolicyOpts {
	return &servicePolicyOpts{rootOpts: parent}
}

func (opts *servicePolicyOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage policies for a service.",
		Example: makeExample(
			"fluxctl policy --service=foo --automate",
			"fluxctl policy --service=foo --lock",
			"fluxctl policy --service=foo --tag='bar=1.*' --tag='baz=2.*'",
		),
		RunE: opts.RunE,
	}

	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	flags := cmd.Flags()
	flags.StringVarP(&opts.service, "service", "s", "", "Service to modify")
	flags.StringSliceVar(&opts.tags, "tag", nil, "Tag filter patterns")
	flags.BoolVar(&opts.automate, "automate", false, "Automate service")
	flags.BoolVar(&opts.deautomate, "deautomate", false, "Deautomate for service")
	flags.BoolVar(&opts.lock, "lock", false, "Lock service")
	flags.BoolVar(&opts.unlock, "unlock", false, "Unlock service")

	return cmd
}

func (opts *servicePolicyOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}
	if opts.automate && opts.deautomate {
		return newUsageError("automate and deautomate both specified")
	}
	if opts.lock && opts.unlock {
		return newUsageError("lock and unlock both specified")
	}

	serviceID, err := flux.ParseServiceID(opts.service)
	if err != nil {
		return err
	}

	update, err := calculatePolicyChanges(opts)
	if err != nil {
		return err
	}
	jobID, err := opts.API.UpdatePolicies(noInstanceID, policy.Updates{
		serviceID: update,
	}, opts.cause)
	if err != nil {
		return err
	}
	return await(cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, false, opts.verbose)
}

func calculatePolicyChanges(opts *servicePolicyOpts) (policy.Update, error) {
	add := policy.Set{}
	if opts.automate {
		add = add.Add(policy.Automated)
	}
	if opts.lock {
		add = add.Add(policy.Locked)
	}

	remove := policy.Set{}
	if opts.deautomate {
		remove = remove.Add(policy.Automated)
	}
	if opts.unlock {
		remove = remove.Add(policy.Locked)
	}

	for _, tagPair := range opts.tags {
		parts := strings.Split(tagPair, "=")
		if len(parts) != 2 {
			return policy.Update{}, fmt.Errorf("invalid container/tag pair: %q", tagPair)
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
