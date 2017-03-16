package diff

import (
	"fmt"
	"io"
)

func (d Chunk) Summarise(out io.Writer) {
	fmt.Fprintf(out, "%s:\n", d.Path)
	for _, del := range d.Deleted {
		fmt.Fprintf(out, "- %#v\n", del)
	}
	for _, add := range d.Added {
		fmt.Fprintf(out, "+ %#v\n", add)
	}
}

func (d OpaqueChunk) Summarise(out io.Writer) {
	fmt.Fprintf(out, "%s: value has changed\n", d.Path)
}
