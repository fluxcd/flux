package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/registry"
)

type serviceShowOpts struct {
	*serviceOpts
	service string
}

func newServiceShow(parent *serviceOpts) *serviceShowOpts {
	return &serviceShowOpts{serviceOpts: parent}
}

func (opts *serviceShowOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "Show the deployed and available images for a service.",
		Example: makeExample("fluxctl list-images --service=helloworld"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Show images for this service")
	return cmd
}

func (opts *serviceShowOpts) RunE(_ *cobra.Command, args []string) error {
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
		runningImage := registry.ParseImage(container.Container.Image)
		fmt.Fprintf(out, "%s\t%s\t\n", containerName, runningImage.Repository())
		foundRunning := false
		for _, image := range container.Images {
			running := "|  "
			if image.Tag == runningImage.Tag {
				running = "'->"
				foundRunning = true
			} else if foundRunning {
				running = "   "
			}
			fmt.Fprintf(out, "\t%s %s\t%s\n", running, image.Tag, image.CreatedAt.Format(time.RFC822))
		}
	}
	out.Flush()
	return nil
}
