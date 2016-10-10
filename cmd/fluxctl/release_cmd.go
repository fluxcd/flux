package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
)

type serviceReleaseOpts struct {
	*serviceOpts
	service     string
	allServices bool
	image       string
	allImages   bool
	noUpdate    bool
	dryRun      bool
	exclude     []string
}

func newServiceRelease(parent *serviceOpts) *serviceReleaseOpts {
	return &serviceReleaseOpts{serviceOpts: parent}
}

func (opts *serviceReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		Example: makeExample(
			"fluxctl release --service=default/foo --update-image=library/hello:v2",
			"fluxctl release --all --update-image=library/hello:v2",
			"fluxctl release --service=default/foo --update-all-images",
			"fluxctl release --service=default/foo --no-update",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "service to release")
	cmd.Flags().BoolVar(&opts.allServices, "all", false, "release all services")
	cmd.Flags().StringVarP(&opts.image, "update-image", "i", "", "update a specific image")
	cmd.Flags().BoolVar(&opts.allImages, "update-all-images", false, "update all images to latest versions")
	cmd.Flags().BoolVar(&opts.noUpdate, "no-update", false, "don't update images; just deploy the service(s) as configured in the git repo")
	cmd.Flags().StringSliceVar(&opts.exclude, "exclude", []string{}, "exclude a service")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "do not release anything; just report back what would have been done")
	return cmd
}

func (opts *serviceReleaseOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if err := checkExactlyOne("--update-image=<image>, --update-all-images, or --no-update",
		opts.image != "", opts.allImages, opts.noUpdate); err != nil {
		return err
	}

	if err := checkExactlyOne("--service=<service>, or --all", opts.service != "", opts.allServices); err != nil {
		return err
	}

	service, err := parseServiceOption(opts.service) // will be "" iff opts.allServices
	if err != nil {
		return err
	}

	var image flux.ImageSpec
	switch {
	case opts.image != "":
		image = flux.ParseImageSpec(opts.image)
	case opts.allImages:
		image = flux.ImageSpecLatest
	case opts.noUpdate:
		image = flux.ImageSpecNone
	}

	var kind flux.ReleaseKind = flux.ReleaseKindExecute
	if opts.dryRun {
		kind = flux.ReleaseKindPlan
	}

	var excludes []flux.ServiceID
	for _, exclude := range opts.exclude {
		s, err := flux.ParseServiceID(exclude)
		if err != nil {
			return err
		}
		excludes = append(excludes, s)
	}

	begin := time.Now()
	printf := func(format string, args ...interface{}) {
		args = append([]interface{}{(int(time.Since(begin).Seconds()))}, args...)
		fmt.Fprintf(os.Stdout, "t=%d "+format+"\n", args...)
	}

	if opts.dryRun {
		printf("Submitting dry-run release job...")
	} else {
		printf("Submitting release job...")
	}

	id, err := opts.Fluxd.PostRelease(noInstanceID, flux.ReleaseJobSpec{
		ServiceSpec: service,
		ImageSpec:   image,
		Kind:        kind,
		Excludes:    excludes,
	})
	if err != nil {
		return err
	}
	printf("Release job submitted, ID %s", id)

	lastStatus := ""
	var job flux.ReleaseJob
	for range time.Tick(time.Second) {
		job, err = opts.Fluxd.GetRelease(noInstanceID, id)
		if err != nil {
			printf("Release errored!")
			return err
		}
		if job.Status != lastStatus {
			lastStatus = job.Status
			printf(job.Status)
		}
		if job.IsFinished() {
			fmt.Println()
			break
		}
	}

	if !job.Success {
		fmt.Fprintf(os.Stdout, "Here's as far as we got:\n")
	} else if opts.dryRun {
		fmt.Fprintf(os.Stdout, "Here's the plan:\n")
	} else {
		fmt.Fprintf(os.Stdout, "Here's what happened:\n")
	}
	for i, msg := range job.Log {
		fmt.Fprintf(os.Stdout, " %d) %s\n", i+1, msg)
	}

	if !opts.dryRun {
		fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	}
	return nil
}
