package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type serviceImagesOpts struct {
	*serviceOpts
	service   string
	namespace string
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
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Namespace in which the service runs.")
	return cmd
}

func (opts *serviceImagesOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("expected no arguments")
	}
	if opts.service == "" {
		return fmt.Errorf("--service flag required")
	}

	containers, err := opts.Fluxd.ServiceImages(opts.namespace, opts.service)
	if err != nil {
		return err
	}

	out := tabwriter.NewWriter(os.Stdout, 4, 4, 2, ' ', 0)
	fmt.Fprintln(out, "CONTAINER\tRUNNING\tIMAGE\tCREATED")
	for _, container := range containers {
		containerName := container.Container.Name
		for _, image := range container.Images {
			running := ""
			imageName := fmt.Sprintf("%s:%s", image.Name, image.Tag)
			if imageName == container.Container.Image {
				running = "--->"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", containerName, running, imageName, image.CreatedAt)
			containerName = "..."
		}
	}
	out.Flush()
	return nil
}
