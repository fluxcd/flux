package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

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
