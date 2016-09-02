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

	begin := time.Now()
	actions, err := opts.Fluxd.Release(service, image, kind)

	for i, action := range actions {
		fmt.Fprintf(os.Stdout, "%d)\t%s\n", i+1, action.Description)
		if action.Result != "" {
			fmt.Fprintln(os.Stdout, "...\t "+action.Result)
		}
	}

	if err != nil {
		return err
	}

	if !opts.dryRun {
		fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	}
	return nil
}
