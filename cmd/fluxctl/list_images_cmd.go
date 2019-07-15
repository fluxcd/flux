package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/resource"
	"github.com/weaveworks/flux/update"
)

type imageListOpts struct {
	*rootOpts
	namespace string
	workload  string
	limit     int

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
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", getKubeConfigContextNamespace(""), "Namespace")
	cmd.Flags().StringVarP(&opts.workload, "workload", "w", "", "Show images for this workload")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 10, "Number of images to show (0 for all)")

	// Deprecated
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Show images for this controller")
	cmd.Flags().MarkDeprecated("controller", "changed to --workload, use that instead")

	return cmd
}

func (opts *imageListOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	imageOpts := v10.ListImagesOptions{
		Spec:      update.ResourceSpecAll,
		Namespace: opts.namespace,
	}
	// Backwards compatibility with --controller until we remove it
	switch {
	case opts.workload != "" && opts.controller != "":
		return newUsageError("can't specify both the controller and workload")
	case opts.controller != "":
		opts.workload = opts.controller
	}
	if len(opts.workload) > 0 {
		id, err := resource.ParseIDOptionalNamespace(opts.namespace, opts.workload)
		if err != nil {
			return err
		}
		imageOpts.Spec = update.MakeResourceSpec(id)
		imageOpts.Namespace = ""
	}

	ctx := context.Background()

	workloads, err := opts.API.ListImagesWithOptions(ctx, imageOpts)
	if err != nil {
		return err
	}

	sort.Sort(imageStatusByName(workloads))

	out := newTabwriter()

	fmt.Fprintln(out, "WORKLOAD\tCONTAINER\tIMAGE\tCREATED")
	for _, workload := range workloads {
		if len(workload.Containers) == 0 {
			fmt.Fprintf(out, "%s\t\t\t\n", workload.ID)
			continue
		}

		workloadName := workload.ID.String()
		for _, container := range workload.Containers {
			var lineCount int
			containerName := container.Name
			reg, repo, currentTag := container.Current.ID.Components()
			if reg != "" {
				reg += "/"
			}
			if len(container.Available) == 0 {
				availableErr := container.AvailableError
				if availableErr == "" {
					availableErr = registry.ErrNoImageData.Error()
				}
				fmt.Fprintf(out, "%s\t%s\t%s%s\t%s\n", workloadName, containerName, reg, repo, availableErr)
			} else {
				fmt.Fprintf(out, "%s\t%s\t%s%s\t\n", workloadName, containerName, reg, repo)
			}
			foundRunning := false
			for _, available := range container.Available {
				running := "|  "
				_, _, tag := available.ID.Components()
				if currentTag == tag {
					running = "'->"
					foundRunning = true
				} else if foundRunning {
					running = "   "
				}

				lineCount++
				var printEllipsis, printLine bool
				if opts.limit <= 0 || lineCount <= opts.limit {
					printEllipsis, printLine = false, true
				} else if container.Current.ID == available.ID {
					printEllipsis, printLine = lineCount > (opts.limit+1), true
				}
				if printEllipsis {
					fmt.Fprintf(out, "\t\t%s (%d image(s) omitted)\t\n", ":", lineCount-opts.limit-1)
				}
				if printLine {
					createdAt := ""
					if !available.CreatedTS().IsZero() {
						createdAt = available.CreatedTS().Format(time.RFC822)
					}
					fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, tag, createdAt)
				}
			}
			if !foundRunning {
				running := "'->"
				if currentTag == "" {
					currentTag = "(untagged)"
				}
				fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, currentTag, "?")

			}
			workloadName = ""
		}
	}
	out.Flush()
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
