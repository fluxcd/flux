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
	service      string
	file         string
	image        string
	updatePeriod time.Duration
}

func newServiceRelease(parent *serviceOpts) *serviceReleaseOpts {
	return &serviceReleaseOpts{serviceOpts: parent}
}

func (opts *serviceReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		Example: makeExample(
			"fluxctl service release --service=helloworld --image=library/hello:1234",
			"fluxctl service release --service=helloworld --file=helloworld-rc.yaml",
			"cat foo-rc.yaml | fluxctl service release -s foo -f -",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "service to update (required)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "file containing new ReplicationController definition, or - to read from stdin")
	cmd.Flags().StringVarP(&opts.image, "image", "i", "", "update the service to a specific image")
	cmd.Flags().DurationVarP(&opts.updatePeriod, "update-period", "p", 5*time.Second, "delay between starting and stopping instances in the rolling update")
	return cmd
}

func (opts *serviceReleaseOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}
	if opts.service == "" {
		return newUsageError("-s, --service is required")
	}

	var exec func() error
	var err error
	switch {
	case opts.image != "" && opts.file != "":
		return newUsageError("cannot have both an -i, --image and -f, --file")

	case opts.image != "":
		exec = func() error {
			return opts.Fluxd.ReleaseImage(opts.namespace, opts.service, opts.image, opts.updatePeriod)
		}

	case opts.file != "":
		var buf []byte
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
		exec = func() error {
			return opts.Fluxd.ReleaseFile(opts.namespace, opts.service, buf, opts.updatePeriod)
		}

	default:
		return newUsageError("one of -i, --image, or -f, --file, are required")

	}

	begin := time.Now()
	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.service, opts.updatePeriod.String())
	if err = exec(); err != nil {
		fmt.Fprintf(os.Stdout, "error! %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
