package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type controllerAutomateOpts struct {
	*rootOpts
	namespace  string
	controller string
	outputOpts
	cause update.Cause

	// Deprecated
	service string
}

func newServiceAutomate(parent *rootOpts) *controllerAutomateOpts {
	return &controllerAutomateOpts{rootOpts: parent}
}

func (opts *controllerAutomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automate",
		Short: "Turn on automatic deployment for a controller.",
		Example: makeExample(
			"fluxctl automate --controller=deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to automate")

	// Deprecated
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to automate")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerAutomateOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}
	policyOpts := &controllerPolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		namespace:  opts.namespace,
		controller: opts.controller,
		cause:      opts.cause,
		automate:   true,
	}
	return policyOpts.RunE(cmd, args)
}
