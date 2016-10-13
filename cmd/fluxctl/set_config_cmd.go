package main

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/fluxy"
)

type setConfigOpts struct {
	*rootOpts
	file string
}

func newSetConfig(parent *rootOpts) *setConfigOpts {
	return &setConfigOpts{rootOpts: parent}
}

func (opts *setConfigOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-config",
		Short: "set configuration values for an instance",
		Example: makeExample(
			"fluxctl config --file=./dev/flux-conf.yaml",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "A file to upload as configuration; this will overwrite all values.")
	return cmd
}

func (opts *setConfigOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	if opts.file == "" {
		return newUsageError("-f, --file is required")
	}

	var config flux.InstanceConfig

	bytes, err := ioutil.ReadFile(opts.file)
	if err == nil {
		err = yaml.Unmarshal(bytes, &config)
	}
	if err != nil {
		return errors.Wrapf(err, "reading config from file")
	}

	return opts.FluxSVC.SetConfig(noInstanceID, config)
}
