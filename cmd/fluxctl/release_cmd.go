package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type serviceReleaseOpts struct {
	*serviceOpts
	service string
	file    string
	image   string
}

func newServiceRelease(parent *serviceOpts) *serviceReleaseOpts {
	return &serviceReleaseOpts{serviceOpts: parent}
}

func (opts *serviceReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		Example: makeExample(
			"fluxctl release --service=helloworld --image=library/hello:1234",
			"fluxctl release --service=helloworld --file=helloworld-rc.yaml",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "service to update (required)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "file containing new ReplicationController definition, or - to read from stdin")
	cmd.Flags().StringVarP(&opts.image, "image", "i", "", "update the service to a specific image")
	return cmd
}

func (opts *serviceReleaseOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}

	if opts.image != "" && opts.file != "" {
		return newUsageError("cannot have both an -i, --image and -f, --file")
	}

	if opts.image == "" && opts.file == "" {
		return newUsageError("one of -i, --image, or -f, --file, are required")
	}

	var buf []byte
	var err error
	if opts.file != "" {
		if opts.file == "-" {
			buf, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
		} else {
			buf, err = ioutil.ReadFile(opts.file)
			if err != nil {
				return err
			}
		}
	}

	begin := time.Now()
	fmt.Fprintf(os.Stdout, "Starting release of %s ...", opts.service)
	if err := opts.Fluxd.Release(opts.namespace, opts.service, opts.image, buf); err != nil {
		fmt.Fprintf(os.Stdout, "error! %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
