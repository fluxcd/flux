package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/weaveworks/fluxy"
)

type configOpts struct {
	*rootOpts
	file    string
	secrets bool
	output  string
}

func newConfig(parent *rootOpts) *configOpts {
	return &configOpts{rootOpts: parent}
}

func (opts *configOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "retrieve or supply configuration for an instance",
		Example: makeExample(
			"fluxctl config --output=yaml",
			"fluxctl config --file=./dev.conf",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "A file to upload as configuration. If omitted, the current config will be shown")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "yaml", `The format to output ("yaml" or "json")`)
	cmd.Flags().BoolVar(&opts.secrets, "secrets", false, "Include secrets when showing current config.")
	return cmd
}

func (opts *configOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errorWantedNoArgs
	}

	if opts.file == "" {

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

		config, err := opts.Fluxd.GetConfig(noInstanceID, opts.secrets)
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

	return uploadConfig(opts.Fluxd, opts.file)
}

func uploadConfig(service flux.Service, path string) error {
	var config flux.InstanceConfig

	bytes, err := ioutil.ReadFile(path)
	if err == nil {
		err = yaml.Unmarshal(bytes, &config)
	}
	if err != nil {
		return errors.Wrapf(err, "reading config from file")
	}

	return service.SetConfig(noInstanceID, config, false)
}
