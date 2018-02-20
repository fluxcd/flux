package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type controllerUnlockOpts struct {
	*rootOpts
	namespace  string
	controller string
	outputOpts
	cause update.Cause

	// Deprecated
	service string
}

func newControllerUnlock(parent *rootOpts) *controllerUnlockOpts {
	return &controllerUnlockOpts{rootOpts: parent}
}

func (opts *controllerUnlockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a controller, so it can be deployed.",
		Example: makeExample(
			"fluxctl unlock --controller=default:deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to unlock")

	// Deprecate
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to unlock")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerUnlockOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}
	policyOpts := &controllerPolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		namespace:  opts.namespace,
		controller: opts.controller,
		cause:      opts.cause,
		unlock:     true,
	}
	return policyOpts.RunE(cmd, args)
}
