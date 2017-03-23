package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flux/diff"
	"github.com/weaveworks/flux/platform/kubernetes/resource"
)

type diffOpts struct {
	*rootOpts
	quiet bool
}

func newDiff(root *rootOpts) *diffOpts {
	return &diffOpts{rootOpts: root}
}

func (opts *diffOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between one platform config and another",
		Example: `# Diff the resource(s) from two files
fluxctl diff dev/resource.yml prod/resource.yml

# Diff the resource(s) in directory dev vs directory prod, recursively
fluxctl diff dev prod
`,
		RunE: opts.RunE,
	}
	cmd.Flags().BoolVarP(&opts.quiet, "quiet", "q", false, "just report which files differ")
	return cmd
}

func (opts *diffOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return newUsageError("please supply two filenames")
	}

	a, err := resource.Load(args[0])
	if err != nil {
		return err
	}
	b, err := resource.Load(args[1])
	if err != nil {
		return err
	}

	diffs, err := diff.DiffSet(a, b)
	if err != nil {
		return err
	}

	output(diffs, opts.quiet, cmd.OutOrStdout())
	return nil
}

func output(d diff.ObjectSetDiff, quiet bool, out io.Writer) {
	if len(d.OnlyA) > 0 {
		for _, obj := range d.OnlyA {
			id := obj.ID()
			fmt.Fprintf(out, "Only in %s: %s %s\n", d.A.Source, id.String(), mustRel(d.A, obj))
		}
	}
	if len(d.OnlyB) > 0 {
		for _, obj := range d.OnlyB {
			id := obj.ID()
			fmt.Fprintf(out, "Only in %s: %s %s\n", d.B.Source, id.String(), mustRel(d.B, obj))
		}
	}
	if len(d.Different) > 0 {
		for id, diffs := range d.Different {
			a, b := d.A.Objects[id], d.B.Objects[id]
			if quiet {
				fmt.Fprintf(out, "Object %s in %s and %s differs\n", id.String(), a.Source(), b.Source())
				continue
			}
			fmt.Fprintf(out, "Object %s %s %s\n", id.String(), a.Source(), b.Source())
			for _, diff := range diffs {
				diff.Summarise(out)
			}
		}
	}
}

func mustRel(objset *diff.ObjectSet, obj diff.Object) string {
	p, err := filepath.Rel(objset.Source, obj.Source())
	if err != nil {
		panic(err)
	}
	return p
}
