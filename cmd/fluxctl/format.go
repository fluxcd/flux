package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"

	"github.com/fluxcd/flux/pkg/registry"

	v6 "github.com/fluxcd/flux/pkg/api/v6"

	"github.com/spf13/cobra"
)

type outputOpts struct {
	verbosity int
}

const (
	outputFormatJson = "json"
	outputFormatTab  = "tab"
)

var validOutputFormats = []string{outputFormatJson, outputFormatTab}

func AddOutputFlags(cmd *cobra.Command, opts *outputOpts) {
	cmd.Flags().CountVarP(&opts.verbosity, "verbose", "v", "include skipped (and ignored, with -vv) workloads in output")
}

func newTabwriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

func makeExample(examples ...string) string {
	var buf bytes.Buffer
	for _, ex := range examples {
		fmt.Fprintf(&buf, "  "+ex+"\n")
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func outputFormatIsValid(format string) bool {
	for _, f := range validOutputFormats {
		if f == format {
			return true
		}
	}
	return false
}

// outputImagesJson sends the provided ImageStatus info to the io.Writer in JSON formatting, honoring limits in opts
func outputImagesJson(images []v6.ImageStatus, out io.Writer, opts *imageListOpts) error {
	if opts.limit < 0 {
		return errors.New("opts.limit cannot be less than 0")
	}
	var sliceLimit int

	// Truncate the Available container images to honor the lesser of
	// opts.limit or the total number of Available
	for i := 0; i < len(images); i++ {
		containerImages := images[i]
		for i := 0; i < len(containerImages.Containers); i++ {
			if opts.limit != 0 {
				available := containerImages.Containers[i].Available
				if len(available) < opts.limit {
					sliceLimit = len(available)
				} else {
					sliceLimit = opts.limit
				}
				containerImages.Containers[i].Available = containerImages.Containers[i].Available[:sliceLimit]
			}
		}
	}

	e := json.NewEncoder(out)
	if err := e.Encode(images); err != nil {
		return err
	}
	return nil
}

// outputImagesTab sends the provided ImageStatus info to os.Stdout in tab formatting, honoring limits in opts
func outputImagesTab(images []v6.ImageStatus, opts *imageListOpts) {
	out := newTabwriter()

	if !opts.noHeaders {
		fmt.Fprintln(out, "WORKLOAD\tCONTAINER\tIMAGE\tCREATED")
	}

	for _, image := range images {
		if len(image.Containers) == 0 {
			fmt.Fprintf(out, "%s\t\t\t\n", image.ID)
			continue
		}

		imageName := image.ID.String()
		for _, container := range image.Containers {
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
				fmt.Fprintf(out, "%s\t%s\t%s%s\t%s\n", imageName, containerName, reg, repo, availableErr)
			} else {
				fmt.Fprintf(out, "%s\t%s\t%s%s\t\n", imageName, containerName, reg, repo)
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
			if !foundRunning {
				running := "'->"
				if currentTag == "" {
					currentTag = "(untagged)"
				}
				fmt.Fprintf(out, "\t\t%s %s\t%s\n", running, currentTag, "?")

			}
			imageName = ""
		}
	}
	out.Flush()
}

// outputWorkloadsJson sends the provided Workload data to the io.Writer as JSON
func outputWorkloadsJson(workloads []v6.ControllerStatus, out io.Writer) error {
	encoder := json.NewEncoder(out)
	return encoder.Encode(workloads)
}

// outputWorkloadsTab sends the provided Workload data to STDOUT, formatted with tabs for CLI
func outputWorkloadsTab(workloads []v6.ControllerStatus, opts *workloadListOpts) {
	w := newTabwriter()
	if !opts.noHeaders {
		fmt.Fprintf(w, "WORKLOAD\tCONTAINER\tIMAGE\tRELEASE\tPOLICY\n")
	}

	for _, workload := range workloads {
		if len(workload.Containers) > 0 {
			c := workload.Containers[0]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", workload.ID, c.Name, c.Current.ID, workload.Status, policies(workload))
			for _, c := range workload.Containers[1:] {
				fmt.Fprintf(w, "\t%s\t%s\t\t\n", c.Name, c.Current.ID)
			}
		} else {
			fmt.Fprintf(w, "%s\t\t\t%s\t%s\n", workload.ID, workload.Status, policies(workload))
		}
	}
	w.Flush()
}
