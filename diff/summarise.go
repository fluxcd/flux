package diff

import (
	"fmt"
	"io"
)

func (d Changed) Summarise(out io.Writer) {
	fmt.Fprintf(out, "* %s: %v != %v\n", d.Path, d.A, d.B)
}

func (d Added) Summarise(out io.Writer) {
	fmt.Fprintf(out, "+ %s: %+v\n", d.Path, d.Value)
}

func (d Removed) Summarise(out io.Writer) {
	fmt.Fprintf(out, "- %s: %+v\n", d.Path, d.Value)
}

func (d OpaqueChanged) Summarise(out io.Writer) {
	fmt.Fprintf(out, "* %s: data has changed\n", d.Path)
}

func (d ObjectSetDiff) Summarise(out io.Writer) {
	if len(d.OnlyA) > 0 {
		fmt.Fprintf(out, "Only in %s:\n", d.A.Source)
		for _, obj := range d.OnlyA {
			id := obj.ID()
			fmt.Fprintf(out, "%s %s/%s (%s)\n", id.Kind, id.Namespace, id.Name, obj.Source())
		}
	}
	if len(d.OnlyB) > 0 {
		fmt.Fprintf(out, "Only in %s:\n", d.B.Source)
		for _, obj := range d.OnlyB {
			id := obj.ID()
			fmt.Fprintf(out, "%s %s/%s (%s)\n", id.Kind, id.Namespace, id.Name, obj.Source())
		}
	}
	if len(d.Different) > 0 {
		for id, diffs := range d.Different {
			fmt.Fprintf(out, "\n%s %s/%s is different:\n", id.Kind, id.Namespace, id.Name)
			for _, diff := range diffs {
				diff.Summarise(out)
			}
		}
	}
}
