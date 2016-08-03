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
		Use:     "show",
		Short:   "Show the release status of a service.",
		Example: makeExample("fluxctl service show --service=helloworld"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Service for which to show the release status")
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

	svc, err := opts.Fluxd.Service(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	fmt.Println("Service:", opts.service)
	fmt.Println("State:", svc.State.State)
	fmt.Println("")

	out := newTabwriter()

	fmt.Fprintln(out, "CONTAINER\tIMAGE\tCREATED")
	for _, container := range containers {
		containerName := container.Container.Name
		runningImage := registry.ParseImage(container.Container.Image)
		fmt.Fprintf(out, "%s\t%s\t\n", containerName, runningImage.Repository())
		foundRunning := runningImage.Tag == ""
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
