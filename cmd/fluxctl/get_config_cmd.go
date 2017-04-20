package main

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
)

type getConfigOpts struct {
	*rootOpts
	fingerprint string
	output      string
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
	cmd.Flags().StringVarP(&opts.output, "output", "o", "yaml", `The format to output ("yaml" or "json")`)
	cmd.Flags().StringVar(&opts.fingerprint, "fingerprint", "", `Show a fingerprint of the public key, using the hash given ("md5" or "sha256")`)
	return cmd
}

func (opts *getConfigOpts) RunE(cmd *cobra.Command, args []string) error {
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

	config, err := opts.API.GetConfig(noInstanceID, opts.fingerprint)
	if err != nil {
		return err
	}

	// Since we always want to output whatever we got, use UnsafeInstanceConfig
	bytes, err := marshal(flux.UnsafeInstanceConfig(config))
	if err != nil {
		return errors.Wrap(err, "marshalling to output format "+opts.output)
	}
	cmd.OutOrStdout().Write(bytes)
	return nil
}
