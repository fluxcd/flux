package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/cobra"
)

type serviceReleaseOpts struct {
	*serviceOpts
	Service      string
	File         string
	UpdatePeriod time.Duration
}

func serviceReleaseCommand(parent *serviceOpts) *serviceReleaseOpts {
	return &serviceReleaseOpts{serviceOpts: parent}
}

func (opts *serviceReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release a new version of a service.",
		RunE:  opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.Service, "service", "s", "", "service to update")
	cmd.Flags().StringVarP(&opts.File, "file", "f", "-", "file containing new ReplicationController definition, or - to read from stdin")
	cmd.Flags().DurationVarP(&opts.UpdatePeriod, "update-period", "p", 5*time.Second, "delay between starting and stopping instances in the rolling update")
	return cmd
}

func (opts *serviceReleaseOpts) RunE(*cobra.Command, []string) error {
	if opts.Service == "" {
		return errors.New("-s, --service is required")
	}

	var buf []byte
	var err error
	switch opts.File {
	case "":
		return errors.New("-f, --file is required")

	case "-":
		buf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

	default:
		buf, err = ioutil.ReadFile(opts.File)
		if err != nil {
			return err
		}
	}

	begin := time.Now()
	fmt.Fprintf(os.Stdout, "Starting release of %s with an update period of %s... ", opts.Service, opts.UpdatePeriod.String())
	if err = opts.Fluxd.Release(opts.Namespace, opts.Service, buf, opts.UpdatePeriod); err != nil {
		fmt.Fprintf(os.Stdout, "error! %v\n", err)
	} else {
		fmt.Fprintf(os.Stdout, "success\n")
	}
	fmt.Fprintf(os.Stdout, "took %s\n", time.Since(begin))
	return nil
}
