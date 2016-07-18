package main

import (
	"os"
	"text/tabwriter"
)

func newTabwriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}
