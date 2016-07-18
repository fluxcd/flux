package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type imagesOpts struct {
	*rootOpts
}

func newImages(parent *rootOpts) *imagesOpts {
	return &imagesOpts{rootOpts: parent}
}

func (opts *imagesOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images <repository>",
		Short: "List images available in an image repository.",
		RunE:  opts.RunE,
	}
	return cmd
}

func (opts *imagesOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf(`expected argument <repository>, e.g. "quay.io/weaveworks/helloworld"`)
	}

	images, err := opts.Fluxd.Images(args[0])
	if err != nil {
		return err
	}

	out := newTabwriter()
	fmt.Fprintln(out, "IMAGE\tCREATED")
	for _, image := range images {
		fmt.Fprintf(out, "%s:%s\t%s\n", image.Name, image.Tag, image.CreatedAt)
	}
	out.Flush()
	return nil
}
