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
			"fluxctl release --service=helloworld --image=library/hello:1234",
			"fluxctl release --all --image=library/hello:1234",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "service to update")
	cmd.Flags().BoolVar(&opts.allServices, "all", false, "release all services")
	cmd.Flags().StringVarP(&opts.image, "update-image", "i", "", "update a specific image")
	cmd.Flags().BoolVar(&opts.allImages, "update-all-images", false, "update all images to latest versions")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "do not release anything; just report back what would have been done")
	return cmd
}

func (opts *serviceReleaseOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}

	if opts.image != "" && opts.allImages {
		return mutuallyExclusive("-i, --update-image", "--update-all-images")
	} else if opts.image == "" && !opts.allImages {
		return exactlyOne("-i, --update-image", "--update-all-images")
	}

	if opts.service != "" && opts.allServices {
		return mutuallyExclusive("-s, --service", "--all")
	} else if opts.service == "" && !opts.allServices {
		return exactlyOne("-s, --service", "--all")
	}

	service, err := parseServiceOption(opts.service) // will be "" iff opts.allServices
	if err != nil {
		return err
	}

	image, err := parseImageOption(opts.image) // will be "" iff opts.allImages
	if err != nil {
		return err
	}

	var kind flux.ReleaseKind = flux.ReleaseKindExecute
	if opts.dryRun {
		kind = flux.ReleaseKindPlan
	}

	begin := time.Now()
	actions, err := opts.Fluxd.Release(service, image, kind)
	if err != nil {
		return err
	}

	for i, action := range actions {
		fmt.Fprintf(os.Stdout, "%d) %s\n", i+1, action.Description)
	}
	if !opts.dryRun {
		fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	}
	return nil
}
