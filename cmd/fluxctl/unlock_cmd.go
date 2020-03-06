package main

import (
	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/update"
)

type workloadUnlockOpts struct {
	*rootOpts
	namespace string
	workload  string
	outputOpts
	cause update.Cause

	// Deprecated
	controller string
}

func newWorkloadUnlock(parent *rootOpts) *workloadUnlockOpts {
	return &workloadUnlockOpts{rootOpts: parent}
}

func (opts *workloadUnlockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a workload, so it can be deployed.",
		Example: makeExample(
			"fluxctl unlock --workload=default:deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Controller namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Controller to unlock")

	// Deprecated
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Controller to unlock")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadUnlockOpts) RunE(cmd *cobra.Command, args []string) error {
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
		unlock:     true,
	}
	return policyOpts.RunE(cmd, args)
}
