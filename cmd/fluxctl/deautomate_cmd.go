package main

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/update"
)

type controllerDeautomateOpts struct {
	*rootOpts
	namespace  string
	controller string
	outputOpts
	cause update.Cause

	// Deprecated
	service string
}

func newControllerDeautomate(parent *rootOpts) *controllerDeautomateOpts {
	return &controllerDeautomateOpts{rootOpts: parent}
}

func (opts *controllerDeautomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deautomate",
		Short: "Turn off automatic deployment for a controller.",
		Example: makeExample(
			"fluxctl deautomate --controller=deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to deautomate")

	// Deprecated
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service to deautomate")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerDeautomateOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}
	policyOpts := &controllerPolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		namespace:  opts.namespace,
		controller: opts.controller,
		cause:      opts.cause,
		deautomate: true,
	}
	return policyOpts.RunE(cmd, args)
}
