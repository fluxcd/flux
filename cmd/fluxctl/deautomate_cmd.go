package main

import (
	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/update"
)

type workloadDeautomateOpts struct {
	*rootOpts
	namespace string
	workload  string
	outputOpts
	cause update.Cause

	// Deprecated
	controller string
}

func newWorkloadDeautomate(parent *rootOpts) *workloadDeautomateOpts {
	return &workloadDeautomateOpts{rootOpts: parent}
}

func (opts *workloadDeautomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deautomate",
		Short: "Turn off automatic deployment for a workload.",
		Example: makeExample(
			"fluxctl deautomate --workload=default:deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Workload namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Workload to deautomate")

	// Deprecated
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to deautomate")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadDeautomateOpts) RunE(cmd *cobra.Command, args []string) error {
	// Backwards compatibility with --controller until we remove it
	switch {
	case opts.workload != "" && opts.controller != "":
		return newUsageError("can't specify both a controller and workload")
	case opts.controller != "":
		opts.workload = opts.controller
	}
	ns := getKubeConfigContextNamespaceOrDefault(opts.namespace, "default", opts.Context)
	policyOpts := &workloadPolicyOpts{
		rootOpts:   opts.rootOpts,
		outputOpts: opts.outputOpts,
		namespace:  ns,
		workload:   opts.workload,
		cause:      opts.cause,
		deautomate: true,
	}
	return policyOpts.RunE(cmd, args)
}
