package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type controllerLockOpts struct {
	*rootOpts
	namespace  string
	controller string
	outputOpts
	cause update.Cause

	// Deprecated
	service string
}

func newControllerLock(parent *rootOpts) *controllerLockOpts {
	return &controllerLockOpts{rootOpts: parent}
}

func (opts *controllerLockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock a controller, so it cannot be deployed.",
		Example: makeExample(
			"fluxctl lock --controller=deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to lock")

	// Deprecated
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to lock")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerLockOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}

	policyOpts := &controllerPolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		namespace:  opts.namespace,
		controller: opts.controller,
		cause:      opts.cause,
		lock:       true,
	}
	return policyOpts.RunE(cmd, args)
}
