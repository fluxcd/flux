package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

type imageListOpts struct {
	*imageOpts
}

func newImageList(parent *imageOpts) *imageListOpts {
	return &imageListOpts{imageOpts: parent}
}

func (opts *imageListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List images available in an image repository.",
		Example: makeExample(
			"fluxctl image list --repo=alpine",
			"fluxctl image list -r quay.io/weaveworks/helloworld",
		),
		RunE: opts.RunE,
	}
	return cmd
}

func (opts *imageListOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.repository == "" {
		return newUsageError("flag --repo is required")
	}

	images, err := opts.Fluxd.Images(opts.repository)
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
