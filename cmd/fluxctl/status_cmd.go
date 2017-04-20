package main

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type statusOpts struct {
	*rootOpts
	output string
}

func newStatus(parent *rootOpts) *statusOpts {
	return &statusOpts{rootOpts: parent}
}

func (opts *statusOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "display current system status",
		Example: makeExample(
			"fluxctl status --output=yaml",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.output, "output", "o", "yaml", `The format to output ("yaml" or "json")`)
	return cmd
}

func (opts *statusOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	var marshal func(interface{}) ([]byte, error)
	switch opts.output {
	case "yaml":
		marshal = yaml.Marshal
	case "json":
		marshal = func(v interface{}) ([]byte, error) {
			return json.MarshalIndent(v, "", "  ")
		}
	default:
		return errors.New("unknown output format " + opts.output)
	}

	status, err := opts.API.Status(noInstanceID)

	if err != nil {
		return err
	}

	bytes, err := marshal(status)
	if err != nil {
		return errors.Wrap(err, "marshalling to output format "+opts.output)
	}
	cmd.OutOrStdout().Write(bytes)
	return nil
}
