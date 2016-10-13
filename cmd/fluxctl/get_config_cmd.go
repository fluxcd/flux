package main

import (
	"encoding/json"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type getConfigOpts struct {
	*rootOpts
	secrets bool
	output  string
}

func newGetConfig(parent *rootOpts) *getConfigOpts {
	return &getConfigOpts{rootOpts: parent}
}

func (opts *getConfigOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-config",
		Short: "display configuration values for an instance",
		Example: makeExample(
			"fluxctl config --output=yaml",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().BoolVar(&opts.secrets, "secrets", false, "Include secrets in the output (e.g., git key)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "yaml", `The format to output ("yaml" or "json")`)
	return cmd
}

func (opts *getConfigOpts) RunE(_ *cobra.Command, args []string) error {
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

	config, err := opts.FluxSVC.GetConfig(noInstanceID, opts.secrets)
	if err != nil {
		return err
	}
	bytes, err := marshal(config)
	if err != nil {
		return errors.Wrap(err, "marshalling to output format "+opts.output)
	}
	os.Stdout.Write(bytes)
	return nil
}
