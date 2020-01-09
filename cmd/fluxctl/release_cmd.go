package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	v11 "github.com/fluxcd/flux/pkg/api/v11"
	v6 "github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/cluster"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

type workloadReleaseOpts struct {
	*rootOpts
	namespace    string
	workloads    []string
	allWorkloads bool
	image        string
	allImages    bool
	exclude      []string
	dryRun       bool
	interactive  bool
	force        bool
	watch        bool
	outputOpts
	cause update.Cause

	// Deprecated
	controllers []string
}

func newWorkloadRelease(parent *rootOpts) *workloadReleaseOpts {
	return &workloadReleaseOpts{rootOpts: parent}
}

func (opts *workloadReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a workload.",
		Example: makeExample(
			"fluxctl release -n default --workload=deployment/foo --update-image=library/hello:v2",
			"fluxctl release --all --update-image=library/hello:v2",
			"fluxctl release --workload=default:deployment/foo --update-all-images",
		),
		RunE: opts.RunE,
	}

	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Workload namespace")
	// Note: we cannot define a shorthand for --workload since it clashes with the shorthand of --watch
	cmd.Flags().StringSliceVarP(&opts.workloads, "workload", "", []string{}, "List of workloads to release <namespace>:<kind>/<name>")
	cmd.Flags().BoolVar(&opts.allWorkloads, "all", false, "Release all workloads")
	cmd.Flags().StringVarP(&opts.image, "update-image", "i", "", "Update a specific image")
	cmd.Flags().BoolVar(&opts.allImages, "update-all-images", false, "Update all images to latest versions")
	cmd.Flags().StringSliceVar(&opts.exclude, "exclude", []string{}, "List of workloads to exclude")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Do not release anything; just report back what would have been done")
	cmd.Flags().BoolVar(&opts.interactive, "interactive", false, "Select interactively which containers to update")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Disregard locks and container image filters (has no effect when used with --all or --update-all-images)")
	cmd.Flags().BoolVarP(&opts.watch, "watch", "w", false, "Watch rollout progress during release")

	// Deprecated
	cmd.Flags().StringSliceVarP(&opts.controllers, "controller", "c", []string{}, "List of controllers to release <namespace>:<kind>/<name>")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *workloadReleaseOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if err := checkExactlyOne("--update-image=<image> or --update-all-images", opts.image != "", opts.allImages); err != nil {
		return err
	}

	// Backwards compatibility with --controller until we remove it
	opts.workloads = append(opts.workloads, opts.controllers...)

	switch {
	case len(opts.workloads) <= 0 && !opts.allWorkloads:
		return newUsageError("please supply either --all, or at least one --workload=<workload>")
	case opts.watch && opts.dryRun:
		return newUsageError("cannot use --watch with --dry-run")
	case opts.force && opts.allWorkloads && opts.allImages:
		return newUsageError("--force has no effect when used with --all and --update-all-images")
	case opts.force && opts.allWorkloads:
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: --force will not ignore locked workloads when used with --all\n")
	case opts.force && opts.allImages:
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: --force will not ignore container image tags when used with --update-all-images\n")
	}

	var workloads []update.ResourceSpec
	ns := getKubeConfigContextNamespaceOrDefault(opts.namespace, "default", opts.Context)

	if opts.allWorkloads {
		workloads = []update.ResourceSpec{update.ResourceSpecAll}
	} else {
		for _, workload := range opts.workloads {
			id, err := resource.ParseIDOptionalNamespace(ns, workload)
			if err != nil {
				return err
			}
			workloads = append(workloads, update.MakeResourceSpec(id))
		}
	}

	var (
		image update.ImageSpec
		err   error
	)
	switch {
	case opts.image != "":
		image, err = update.ParseImageSpec(opts.image)
		if err != nil {
			return err
		}
	case opts.allImages:
		image = update.ImageSpecLatest
	}

	var kind update.ReleaseKind = update.ReleaseKindExecute
	if opts.dryRun || opts.interactive {
		kind = update.ReleaseKindPlan
	}

	var excludes []resource.ID
	for _, exclude := range opts.exclude {
		s, err := resource.ParseIDOptionalNamespace(ns, exclude)
		if err != nil {
			return err
		}
		excludes = append(excludes, s)
	}

	if kind == update.ReleaseKindPlan {
		fmt.Fprintf(cmd.OutOrStderr(), "Submitting dry-run release ...\n")
	} else {
		fmt.Fprintf(cmd.OutOrStderr(), "Submitting release ...\n")
	}

	ctx := context.Background()
	spec := update.ReleaseImageSpec{
		ServiceSpecs: workloads,
		ImageSpec:    image,
		Kind:         kind,
		Excludes:     excludes,
		Force:        opts.force,
	}
	jobID, err := opts.API.UpdateManifests(ctx, update.Spec{
		Type:  update.Images,
		Cause: opts.cause,
		Spec:  spec,
	})
	if err != nil {
		return err
	}

	result, err := awaitJob(ctx, opts.API, jobID, opts.Timeout)
	if err != nil {
		return err
	}
	if opts.interactive {
		spec, err := promptSpec(cmd.OutOrStdout(), result, opts.verbosity)
		spec.Force = opts.force
		if err != nil {
			fmt.Fprintln(cmd.OutOrStderr(), err.Error())
			return nil
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Submitting selected release ...\n")
		jobID, err = opts.API.UpdateManifests(ctx, update.Spec{
			Type:  update.Containers,
			Cause: opts.cause,
			Spec:  spec,
		})
		if err != nil {
			fmt.Fprintln(cmd.OutOrStderr(), err.Error())
			return nil
		}

		opts.dryRun = false
	}

	err = await(ctx, cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, !opts.dryRun, opts.verbosity, opts.Timeout)
	if !opts.watch || err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Monitoring rollout ...\n")
	for {
		completed := 0
		workloads, err := opts.API.ListServicesWithOptions(ctx, v11.ListServicesOptions{Services: result.Result.AffectedResources()})
		if err != nil {
			return err
		}

		for _, workload := range workloads {
			writeRolloutStatus(workload, opts.verbosity)

			if workload.Status == cluster.StatusReady {
				completed++
			}

			if workload.Rollout.Messages != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "There was a problem releasing %s:\n", workload.ID)
				for _, msg := range workload.Rollout.Messages {
					fmt.Fprintf(cmd.OutOrStderr(), "%s\n", msg)
				}
				return nil
			}
		}

		if completed == len(workloads) {
			fmt.Fprintf(cmd.OutOrStderr(), "All workloads ready.\n")
			return nil
		}

		time.Sleep(2000 * time.Millisecond)
	}
}

func writeRolloutStatus(workload v6.ControllerStatus, verbosity int) {
	w := newTabwriter()
	fmt.Fprintf(w, "WORKLOAD\tCONTAINER\tIMAGE\tRELEASE\tREPLICAS\n")

	if len(workload.Containers) > 0 {
		c := workload.Containers[0]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d/%d", workload.ID, c.Name, c.Current.ID, workload.Status, workload.Rollout.Updated, workload.Rollout.Desired)
		if verbosity > 0 {
			fmt.Fprintf(w, " (%d outdated, %d ready)", workload.Rollout.Outdated, workload.Rollout.Ready)
		}
		fmt.Fprintf(w, "\n")
		for _, c := range workload.Containers[1:] {
			fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
		}
	} else {
		fmt.Fprintf(w, "%s\t\t\t%s\t%d/%d", workload.ID, workload.Status, workload.Rollout.Updated, workload.Rollout.Desired)
		if verbosity > 0 {
			fmt.Fprintf(w, " (%d outdated, %d ready)", workload.Rollout.Outdated, workload.Rollout.Ready)
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintln(w)
	w.Flush()
}

func promptSpec(out io.Writer, result job.Result, verbosity int) (update.ReleaseContainersSpec, error) {
	menu := update.NewMenu(out, result.Result, verbosity)
	containerSpecs, err := menu.Run()
	return update.ReleaseContainersSpec{
		Kind:           update.ReleaseKindExecute,
		ContainerSpecs: containerSpecs,
		SkipMismatches: false,
	}, err
}
