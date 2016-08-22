package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy"
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

	service, err := parseServiceOption(opts.service)
	if err != nil {
		return err
	}

	services, err := opts.Fluxd.ListImages(service)
	if err != nil {
		return err
	}

	out := newTabwriter()

	fmt.Fprintln(out, "SERVICE\tCONTAINER\tIMAGE\tCREATED")
	for _, service := range services {
		if len(service.Containers) == 0 {
			fmt.Fprintf(out, "%s\t\t\t\n", service.ID)
			continue
		}

		sname := service.ID
		for _, container := range service.Containers {
			containerName := container.Name
			runningImage := parseImageID(container.Current.ID)
			fmt.Fprintf(out, "%s\t%s\t%s\t\n", sname, containerName, runningImage.Repository())
			foundRunning := false
			for _, available := range container.Available {
				running := "|  "
				if container.Current.ID == available.ID {
					running = "'->"
					foundRunning = true
				} else if foundRunning {
					running = "   "
				}
				image := parseImageID(available.ID)
				fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, image.Tag, image.CreatedAt.Format(time.RFC822))
			}
			sname = ""
		}
	}
	out.Flush()
	return nil
}

func parseImageID(id flux.ImageID) registry.Image {
	return registry.ParseImage(string(id))
}
