package main

import (
	"github.com/spf13/cobra"

	"github.com/fluxcd/flux/pkg/update"
)

type workloadLockOpts struct {
	*rootOpts
	namespace string
	workload  string
	outputOpts
	cause update.Cause

	// Deprecated
	controller string
}

func newWorkloadLock(parent *rootOpts) *workloadLockOpts {
	return &workloadLockOpts{rootOpts: parent}
}

func (opts *workloadLockOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock a workload, so it cannot be deployed.",
		Example: makeExample(
			"fluxctl lock --workload=default:deployment/helloworld",
		),
		RunE: opts.RunE,
	}
	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Controller namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Workload to lock")

	// Deprecated
	cmd.Flags().StringVarP(&opts.workload, "controller", "c", "", "Controller to lock")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadLockOpts) RunE(cmd *cobra.Command, args []string) error {
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
		lock:       true,
	}
	return policyOpts.RunE(cmd, args)
}
