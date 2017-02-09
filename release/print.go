package release

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/weaveworks/flux"
)

func PrintResults(results flux.ReleaseResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE \tSTATUS \tUPDATES")
	currentID := ""
	for serviceID, result := range results {
		id := serviceID.String()
		if id == currentID {
			id = ""
		} else {
			currentID = id
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", id, result.Status, result.Error)
		for _, update := range result.PerContainer {
			fmt.Fprintf(w, " \t \t %s: %s -> %s\n", update.Container, update.Current.FullID(), update.Target.Tag)
		}
	}
	w.Flush()
}
