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
