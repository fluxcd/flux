package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type outputOpts struct {
	verbosity int
}

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
