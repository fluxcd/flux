package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/history"
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

	histories, err := opts.Fluxd.History(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	fmt.Println("Service:", opts.service)
	state := history.StateUnknown
	if h, found := histories[opts.service]; found {
		state = h.State
	}
	fmt.Println("State:", state)
	fmt.Println("")

	out := newTabwriter()

	fmt.Fprintln(out, "CONTAINER\tIMAGE\tCREATED")
	for _, container := range containers {
		containerName := container.Container.Name
		_, imageName, imageTag := registry.ImageParts(container.Container.Image)
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
			fmt.Fprintf(out, "\t%s %s\t%s\n", running, image.Tag, image.CreatedAt.Format(time.RFC822))
		}
	}
	out.Flush()
	return nil
}
