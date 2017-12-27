package update

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/weaveworks/flux"
)

// PrintResults outputs a result set to the `io.Writer` provided, at
// the given level of verbosity:
//  - 2 = include skipped and ignored resources
//  - 1 = include skipped resources, exclude ignored resources
//  - 0 = exclude skipped and ignored resources
func PrintResults(out io.Writer, results Result, verbosity int) {
	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "CONTROLLER \tSTATUS \tUPDATES")
	for _, serviceID := range results.ServiceIDs() {
		result := results[flux.MustParseResourceID(serviceID)]
		switch result.Status {
		case ReleaseStatusIgnored:
			if verbosity < 2 {
				continue
			}
		case ReleaseStatusSkipped:
			if verbosity < 1 {
				continue
			}
		}

		var extraLines []string
		if result.Error != "" {
			extraLines = append(extraLines, result.Error)
		}
		for _, update := range result.PerContainer {
			extraLines = append(extraLines, fmt.Sprintf("%s: %s -> %s", update.Container, update.Current.String(), update.Target.Tag))
		}

		var inline string
		if len(extraLines) > 0 {
			inline = extraLines[0]
			extraLines = extraLines[1:]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", serviceID, result.Status, inline)
		for _, lines := range extraLines {
			fmt.Fprintf(w, "\t\t%s\n", lines)
		}
	}
	w.Flush()
}
