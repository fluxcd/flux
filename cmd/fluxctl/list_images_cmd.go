package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
)

type serviceShowOpts struct {
	*serviceOpts
	service string
	limit   int
}

func newServiceShow(parent *serviceOpts) *serviceShowOpts {
	return &serviceShowOpts{serviceOpts: parent}
}

func (opts *serviceShowOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-images",
		Short:   "Show the deployed and available images for a service.",
		Example: makeExample("fluxctl list-images --service=default/foo"),
		RunE:    opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "Show images for this service")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 10, "Number of images to show (0 for all)")
	return cmd
}

func (opts *serviceShowOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	service, err := parseServiceOption(opts.service)
	if err != nil {
		return err
	}

	services, err := opts.API.ListImages(noInstanceID, service)
	if err != nil {
		return err
	}

	sort.Sort(imageStatusByName(services))

	out := newTabwriter()

	fmt.Fprintln(out, "SERVICE\tCONTAINER\tIMAGE\tCREATED")
	for _, service := range services {
		if len(service.Containers) == 0 {
			fmt.Fprintf(out, "%s\t\t\t\n", service.ID)
			continue
		}

		serviceName := service.ID
		var lineCount int
		for _, container := range service.Containers {
			containerName := container.Name
			reg, repo, currentTag := container.Current.ID.Components()
			if reg != "" {
				reg += "/"
			}
			if len(container.Available) == 0 {
				fmt.Fprintf(out, "%s\t%s\t%s%s\twaiting for cache\n", serviceName, containerName, reg, repo)
			} else {
				fmt.Fprintf(out, "%s\t%s\t%s%s\t\n", serviceName, containerName, reg, repo)
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
					fmt.Fprintf(out, "\t\t%s\t\n", ":")
				}
				if printLine {
					createdAt := ""
					if !available.CreatedAt.IsZero() {
						createdAt = available.CreatedAt.Format(time.RFC822)
					}
					fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, tag, createdAt)
				}
			}
			serviceName = ""
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
	return s[a].ID < s[b].ID
}

func (s imageStatusByName) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}
