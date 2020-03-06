package main

import (
	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/update"
)

type workloadAutomateOpts struct {
	*rootOpts
	namespace string
	workload  string
	outputOpts
	cause update.Cause

	// Deprecated
	controller string
}

func newWorkloadAutomate(parent *rootOpts) *workloadAutomateOpts {
	return &workloadAutomateOpts{rootOpts: parent}
}

func (opts *workloadAutomateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "automate",
		Short: "Turn on automatic deployment for a workload.",
		Example: makeExample(
			"fluxctl automate --workload=default:deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Workload namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Workload to automate")

	// Deprecated
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to automate")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadAutomateOpts) RunE(cmd *cobra.Command, args []string) error {
	// Backwards compatibility with --controller until we remove it
	switch {
	case opts.workload != "" && opts.controller != "":
		return newUsageError("can't specify both the controller and workload")
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
		automate:   true,
	}
	return policyOpts.RunE(cmd, args)
}
