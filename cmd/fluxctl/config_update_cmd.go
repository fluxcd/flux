package main

import (
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/weaveworks/fluxy/platform/kubernetes"
)

type configUpdateOpts struct {
	*configOpts
	file      string
	output    string
	image     string
	showTrace bool
}

func newConfigUpdate(parent *configOpts) *configUpdateOpts {
	return &configUpdateOpts{configOpts: parent}
}

func (opts *configUpdateOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update a config file with a new image",
		Example: makeExample(
			"fluxctl config update --file=rc.yaml --image=quay.io/weaveworks/helloworld:de9f3b2 --output=rc.yaml",
			"cat rc.yaml | fluxctl config update -v -i quay.io/weaveworks/helloworld:de9f3b2 > rc.yaml",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.file, "file", "f", "-", `the file to read (or "-" to read from stdin)`)
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "the file to write (stdout if not supplied)")
	cmd.Flags().StringVarP(&opts.image, "image", "i", "", "the new image")
	cmd.Flags().BoolVarP(&opts.showTrace, "verbose", "v", false, "output a trace to stderr")
	return cmd
}

func (opts *configUpdateOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.image == "" {
		return newUsageError("--image is required")
	}
	trace := ioutil.Discard
	if opts.showTrace {
		trace = os.Stderr
	}

	var buf []byte
	var err error
	switch opts.file {
	case "":
		return newUsageError("-f, --file is required")

	case "-":
		buf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

	default:
		buf, err = ioutil.ReadFile(opts.file)
		if err != nil {
			return err
		}
	}

	if opts.output != "" {
		f, err := ioutil.TempFile("", "fluxctl-config-update")
		if err != nil {
			return err
		}
		defer func() { f.Close(); os.Remove(f.Name()) }()

		newbuf, err := kubernetes.UpdateReplicationController(buf, opts.image, trace)
		if err != nil {
			return err
		}
		if _, err := f.Write(newbuf); err != nil {
			return err
		}
		return os.Rename(f.Name(), opts.output)
	}

	newbuf, err := kubernetes.UpdateReplicationController(buf, opts.image, trace)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(newbuf)
	return err
}
