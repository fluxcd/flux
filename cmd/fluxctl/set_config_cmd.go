package main

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/flux"
)

type setConfigOpts struct {
	*rootOpts
	file string
	key  bool
}

func newSetConfig(parent *rootOpts) *setConfigOpts {
	return &setConfigOpts{rootOpts: parent}
}

func (opts *setConfigOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-config",
		Short: "set configuration values for an instance",
		Example: makeExample(
			"fluxctl set-config --file=./dev/flux-conf.yaml --generate-deploy-key",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "A file to upload as configuration; this will overwrite all values.")
	cmd.Flags().BoolVarP(&opts.key, "generate-deploy-key", "k", false, "Generate and replace Git deploy key")
	return cmd
}

func (opts *setConfigOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	if cmd.Flags().NFlag() == 0 {
		return newUsageError("a flag is required")
	}

	if opts.key {
		err := opts.GitGenerateKey()
		if err != nil {
			return err
		}
	}

	if opts.file != "" {
		var config flux.UnsafeInstanceConfig

		bytes, err := ioutil.ReadFile(opts.file)
		if err == nil {
			err = yaml.Unmarshal(bytes, &config)
		}
		if err != nil {
			return errors.Wrapf(err, "reading config from file")
		}

		err = opts.API.SetConfig(noInstanceID, config)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opts *setConfigOpts) GitGenerateKey() error {
	return opts.API.GenerateDeployKey(noInstanceID)
}
