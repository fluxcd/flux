package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type serviceImagesOpts struct {
	*serviceOpts
	service string
}

func newServiceImages(parent *serviceOpts) *serviceImagesOpts {
	return &serviceImagesOpts{serviceOpts: parent}
}

func (opts *serviceImagesOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "List the images available to run for a service.",
		RunE:  opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service for which to show images")
	return cmd
}

func (opts *serviceImagesOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("--service flag required")
	}

	containers, err := opts.Fluxd.ServiceImages(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	out := newTabwriter()
	fmt.Fprintln(out, "CONTAINER\tIMAGE\tCREATED")
	for _, container := range containers {
		containerName := container.Container.Name
		imageName, imageTag := imageParts(container.Container.Image)
		fmt.Fprintf(out, "%s\t%s\t\n", containerName, imageName)
		foundRunning := false
		for _, image := range container.Images {
			running := "|  "
			if image.Tag == imageTag {
				running = "'->"
				foundRunning = true
			} else if foundRunning {
				running = "   "
			}
			fmt.Fprintf(out, "\t%s %s\t%s\n", running, image.Tag, image.CreatedAt)
		}
	}
	out.Flush()
	return nil
}
