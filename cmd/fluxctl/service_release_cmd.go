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
			"fluxctl service release --all --image=library/hello:1234",
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

	if !opts.all && opts.service == "" {
		return newUsageError("one of -a, --all, or -s, --service, are required")
	}

	if opts.image != "" && opts.file != "" {
		return newUsageError("cannot have both -i, --image and -f, --file")
	}

	if opts.image == "" && opts.file == "" {
		return newUsageError("one of -i, --image, or -f, --file, are required")
	}

	if opts.all {
		if opts.service != "" {
			return newUsageError("cannot have both -a, --all and -s, --service")
		}
		if opts.file != "" {
			return newUsageError("cannot have both -a, --all and -f, --file")
		}

		begin := time.Now()
		fmt.Fprintf(os.Stdout, "Starting release of all services with an update period of %s... \n", opts.updatePeriod.String())
		if err := opts.Fluxd.ReleaseAll(opts.namespace, opts.image, opts.updatePeriod); err != nil {
			fmt.Fprintf(os.Stdout, "error!\n%v\n", err)
		} else {
			fmt.Fprintf(os.Stdout, "success\n")
		}
		fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
		return nil
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
	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.service, opts.updatePeriod.String())
	if err := opts.Fluxd.Release(opts.namespace, opts.service, opts.image, buf, opts.updatePeriod); err != nil {
		fmt.Fprintf(os.Stdout, "error!\n%v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
