package main

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/registry"
	"github.com/weaveworks/flux/update"
)

type controllerShowOpts struct {
	*rootOpts
	namespace  string
	controller string
	limit      int

	// Deprecated
	service string
}

func newControllerShow(parent *rootOpts) *controllerShowOpts {
	return &controllerShowOpts{rootOpts: parent}
}

func (opts *controllerShowOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "Show the deployed and available images for a controller.",
		Example: makeExample("fluxctl list-images --namespace default --controller=deployment/foo"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.namespace, "namespace", "n", "default", "Controller namespace")
	cmd.Flags().StringVarP(&opts.controller, "controller", "c", "", "Show images for this controller")
	cmd.Flags().IntVarP(&opts.limit, "limit", "l", 10, "Number of images to show (0 for all)")

	// Deprecated
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Show images for this service")
	cmd.Flags().MarkHidden("service")

	return cmd
}

func (opts *controllerShowOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(opts.service) > 0 {
		return errorServiceFlagDeprecated
	}
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	var resourceSpec update.ResourceSpec
	if len(opts.controller) == 0 {
		resourceSpec = update.ResourceSpecAll
	} else {
		id, err := flux.ParseResourceIDOptionalNamespace(opts.namespace, opts.controller)
		if err != nil {
			return err
		}
		resourceSpec = update.MakeResourceSpec(id)
	}

	ctx := context.Background()

	controllers, err := opts.API.ListImages(ctx, resourceSpec)
	if err != nil {
		return err
	}

	sort.Sort(imageStatusByName(controllers))

	out := newTabwriter()

	fmt.Fprintln(out, "CONTROLLER\tCONTAINER\tIMAGE\tCREATED")
	for _, controller := range controllers {
		if len(controller.Containers) == 0 {
			fmt.Fprintf(out, "%s\t\t\t\n", controller.ID)
			continue
		}

		controllerName := controller.ID.String()
		for _, container := range controller.Containers {
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
				fmt.Fprintf(out, "%s\t%s\t%s%s\t%s\n", controllerName, containerName, reg, repo, availableErr)
			} else {
				fmt.Fprintf(out, "%s\t%s\t%s%s\t\n", controllerName, containerName, reg, repo)
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
					if !available.CreatedAt.IsZero() {
						createdAt = available.CreatedAt.Format(time.RFC822)
					}
					fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, tag, createdAt)
				}
			}
			controllerName = ""
		}
	}
	out.Flush()
	return nil
}

type imageStatusByName []flux.ImageStatus

func (s imageStatusByName) Len() int {
	return len(s)
}

func (s imageStatusByName) Less(a, b int) bool {
	return s[a].ID.String() < s[b].ID.String()
}

func (s imageStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}
