package main

import (
	"context"
	"os"
	"sort"

	"github.com/spf13/cobra"

	v10 "github.com/fluxcd/flux/pkg/api/v10"
	v6 "github.com/fluxcd/flux/pkg/api/v6"
	"github.com/fluxcd/flux/pkg/resource"
	"github.com/fluxcd/flux/pkg/update"
)

type imageListOpts struct {
	*rootOpts
	namespace    string
	workload     string
	limit        int
	noHeaders    bool
	outputFormat string

	// Deprecated
	controller string
}

func newImageList(parent *rootOpts) *imageListOpts {
	return &imageListOpts{rootOpts: parent}
}

func (opts *imageListOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "Show deployed and available images.",
		Example: makeExample("fluxctl list-images --namespace default --workload=deployment/foo"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "", "Namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Show images for this workload")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 10, "Number of images to show (0 for all)")
	cmd.Flags().BoolVar(&opts.noHeaders, "no-headers", false, "Don't print headers (default print headers)")
	cmd.Flags().StringVarP(&opts.outputFormat, "output-format", "o", "tab", "Output format (tab or json)")

	// Deprecated
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Show images for this controller")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *imageListOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if !outputFormatIsValid(opts.outputFormat) {
		return errorInvalidOutputFormat
	}

	ns := getKubeConfigContextNamespaceOrDefault(opts.namespace, "default", opts.Context)
	imageOpts := v10.ListImagesOptions{
		Spec:      update.ResourceSpecAll,
		Namespace: ns,
	}
	// Backwards compatibility with --controller until we remove it
	switch {
	case opts.workload != "" && opts.controller != "":
		return newUsageError("can't specify both the controller and image")
	case opts.controller != "":
		opts.workload = opts.controller
	}
	if len(opts.workload) > 0 {
		id, err := resource.ParseIDOptionalNamespace(ns, opts.workload)
		if err != nil {
			return err
		}
		imageOpts.Spec = update.MakeResourceSpec(id)
		imageOpts.Namespace = ""
	}

	ctx := context.Background()

	images, err := opts.API.ListImagesWithOptions(ctx, imageOpts)
	if err != nil {
		return err
	}

	sort.Sort(imageStatusByName(images))

	switch opts.outputFormat {
	case outputFormatJson:
		return outputImagesJson(images, os.Stdout, opts)
	default:
		outputImagesTab(images, opts)
	}

	return nil
}

type imageStatusByName []v6.ImageStatus

func (s imageStatusByName) Len() int {
	return len(s)
}

func (s imageStatusByName) Less(a, b int) bool {
	return s[a].ID.String() < s[b].ID.String()
}

func (s imageStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}
