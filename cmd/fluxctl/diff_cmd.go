package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"github.com/weaveworks/flux/diff"
	"github.com/weaveworks/flux/platform/kubernetes/resource"
)

type diffOpts struct {
	*rootOpts
	format string
}

func newDiff(root *rootOpts) *diffOpts {
	return &diffOpts{rootOpts: root}
}

func (opts *diffOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show differences between one platform config and another",
		RunE:  opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.format, "output", "o", "text", "(yaml|json|text) whether to output differences in YAML or JSON, or just summarise in text")
	return cmd
}

func (opts *diffOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return newUsageError("please supply two filenames")
	}

	output := func(diffs diff.ObjectSetDiff) error {
		diffs.Summarise(cmd.OutOrStdout())
		return nil
	}

	switch opts.format {
	case "text":
		// already output
	case "raw":
		output = func(diffs diff.ObjectSetDiff) error {
			fmt.Fprintf(cmd.OutOrStdout(), "%#v\n", diffs)
			return nil
		}
	case "json":
		output = func(diffs diff.ObjectSetDiff) error {
			bytes, err := json.Marshal(diffs)
			if err != nil {
				return err
			}
			cmd.OutOrStdout().Write(bytes)
			return nil
		}
	case "yaml":
		output = func(diffs diff.ObjectSetDiff) error {
			bytes, err := yaml.Marshal(diffs)
			if err != nil {
				return err
			}
			cmd.OutOrStdout().Write(bytes)
			return nil
		}
	default:
		return newUsageError("output format --output,-o must be 'raw', 'text', 'json' or 'yaml'")
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

	return output(diffs)
}
