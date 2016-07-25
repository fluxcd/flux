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
			"fluxctl service release --service=helloworld --file=helloworld-rc.yaml",
			"cat foo-rc.yaml | fluxctl service release -s foo",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.service, "service", "s", "", "service to update (required)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "-", "file containing new ReplicationController definition, or - to read from stdin (required)")
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

	begin := time.Now()
	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.service, opts.updatePeriod.String())
	if err = opts.Fluxd.Release(opts.namespace, opts.service, buf, opts.updatePeriod); err != nil {
		fmt.Fprintf(os.Stdout, "error! %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
