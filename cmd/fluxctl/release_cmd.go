package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/cluster"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type controllerReleaseOpts struct {
	*rootOpts
	namespace      string
	controllers    []string
	allControllers bool
	image          string
	allImages      bool
	exclude        []string
	dryRun         bool
	interactive    bool
	force          bool
	watch          bool
	outputOpts
	cause update.Cause

	// Deprecated
	services []string
}

func newControllerRelease(parent *rootOpts) *controllerReleaseOpts {
	return &controllerReleaseOpts{rootOpts: parent}
}

func (opts *controllerReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a controller.",
		Example: makeExample(
			"fluxctl release -n default --controller=deployment/foo --update-image=library/hello:v2",
			"fluxctl release --all --update-image=library/hello:v2",
			"fluxctl release --controller=default:deployment/foo --update-all-images",
		),
		RunE: opts.RunE,
	}

	AddOutputFlags(cmd, &opts.outputOpts)
	AddCauseFlags(cmd, &opts.cause)
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringSliceVarP(&opts.controllers, "controller", "c", []string{}, "List of controllers to release <namespace>:<kind>/<name>")
	cmd.Flags().BoolVar(&opts.allControllers, "all", false, "Release all controllers")
	cmd.Flags().StringVarP(&opts.image, "update-image", "i", "", "Update a specific image")
	cmd.Flags().BoolVar(&opts.allImages, "update-all-images", false, "Update all images to latest versions")
	cmd.Flags().StringSliceVar(&opts.exclude, "exclude", []string{}, "List of controllers to exclude")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Do not release anything; just report back what would have been done")
	cmd.Flags().BoolVar(&opts.interactive, "interactive", false, "Select interactively which containers to update")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Disregard locks and container image filters (has no effect when used with --all or --update-all-images)")
	cmd.Flags().BoolVarP(&opts.watch, "watch", "w", false, "Watch rollout progress during release")

	// Deprecated
	cmd.Flags().StringSliceVarP(&opts.services, "service", "s", []string{}, "Service to release")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerReleaseOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.services) > 0 {
		return errorServiceFlagDeprecated
	}

	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if err := checkExactlyOne("--update-image=<image> or --update-all-images", opts.image != "", opts.allImages); err != nil {
		return err
	}

	switch {
	case len(opts.controllers) <= 0 && !opts.allControllers:
		return newUsageError("please supply either --all, or at least one --controller=<controller>")
	case opts.watch && opts.dryRun:
		return newUsageError("cannot use --watch with --dry-run")
	case opts.force && opts.allControllers && opts.allImages:
		return newUsageError("--force has no effect when used with --all and --update-all-images")
	case opts.force && opts.allControllers:
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: --force will not ignore locked controllers when used with --all\n")
	case opts.force && opts.allImages:
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: --force will not ignore container image tags when used with --update-all-images\n")
	}

	var controllers []update.ResourceSpec

	if opts.allControllers {
		controllers = []update.ResourceSpec{update.ResourceSpecAll}
	} else {
		for _, controller := range opts.controllers {
			id, err := flux.ParseResourceIDOptionalNamespace(opts.namespace, controller)
			if err != nil {
				return err
			}
			controllers = append(controllers, update.MakeResourceSpec(id))
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

	var excludes []flux.ResourceID
	for _, exclude := range opts.exclude {
		s, err := flux.ParseResourceIDOptionalNamespace(opts.namespace, exclude)
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
		ServiceSpecs: controllers,
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

	result, err := awaitJob(ctx, opts.API, jobID)
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

	err = await(ctx, cmd.OutOrStdout(), cmd.OutOrStderr(), opts.API, jobID, !opts.dryRun, opts.verbosity)
	if !opts.watch || err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Monitoring rollout ...\n")
	for {
		completed := 0
		services, err := opts.API.ListServicesWithOptions(ctx, v11.ListServicesOptions{Services: result.Result.AffectedResources()})
		if err != nil {
			return err
		}

		for _, service := range services {
			writeRolloutStatus(service, opts.verbosity)

			if service.Status == cluster.StatusReady {
				completed++
			}

			if service.Rollout.Messages != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "There was a problem releasing %s:\n", service.ID)
				for _, msg := range service.Rollout.Messages {
					fmt.Fprintf(cmd.OutOrStderr(), "%s\n", msg)
				}
				return nil
			}
		}

		if completed == len(services) {
			fmt.Fprintf(cmd.OutOrStderr(), "All controllers ready.\n")
			return nil
		}

		time.Sleep(2000 * time.Millisecond)
	}
}

func writeRolloutStatus(service v6.ControllerStatus, verbosity int) {
	w := newTabwriter()
	fmt.Fprintf(w, "CONTROLLER\tCONTAINER\tIMAGE\tRELEASE\tREPLICAS\n")

	if len(service.Containers) > 0 {
		c := service.Containers[0]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d/%d", service.ID, c.Name, c.Current.ID, service.Status, service.Rollout.Updated, service.Rollout.Desired)
		if verbosity > 0 {
			fmt.Fprintf(w, " (%d outdated, %d ready)", service.Rollout.Outdated, service.Rollout.Ready)
		}
		fmt.Fprintf(w, "\n")
		for _, c := range service.Containers[1:] {
			fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
		}
	} else {
		fmt.Fprintf(w, "%s\t\t\t%s\t%d/%d", service.ID, service.Status, service.Rollout.Updated, service.Rollout.Desired)
		if verbosity > 0 {
			fmt.Fprintf(w, " (%d outdated, %d ready)", service.Rollout.Outdated, service.Rollout.Ready)
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
