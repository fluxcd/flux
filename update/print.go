package update

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/weaveworks/flux"
)

func PrintResults(out io.Writer, results Result, verbose bool) {
	w := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE \tSTATUS \tUPDATES")
	for _, serviceID := range results.ServiceIDs() {
		result := results[flux.ServiceID(serviceID)]
		switch result.Status {
		case ReleaseStatusIgnored:
			if !verbose {
				continue
			}
		}

		var extraLines []string
		if result.Error != "" {
			extraLines = append(extraLines, result.Error)
		}
		for _, update := range result.PerContainer {
			extraLines = append(extraLines, fmt.Sprintf("%s: %s -> %s", update.Container, update.Current.FullID(), update.Target.Tag))
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
