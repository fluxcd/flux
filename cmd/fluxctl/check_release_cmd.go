package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gosuri/uilive"
	"github.com/spf13/cobra"

	flux "github.com/weaveworks/fluxy"
)

const largestHeartbeatDelta = 5 * time.Second

type serviceCheckReleaseOpts struct {
	*serviceOpts
	releaseID string
	follow    bool
}

func newServiceCheckRelease(parent *serviceOpts) *serviceCheckReleaseOpts {
	return &serviceCheckReleaseOpts{serviceOpts: parent}
}

func (opts *serviceCheckReleaseOpts) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-release",
		Short: "Check the status of a release.",
		Example: makeExample(
			"fluxctl check-release --release-id=12345678-1234-5678-1234-567812345678",
		),
		RunE: opts.RunE,
	}
	cmd.Flags().StringVarP(&opts.releaseID, "release-id", "r", "", "release ID to check")
	cmd.Flags().BoolVarP(&opts.follow, "follow", "f", true, "continuously check the release, blocking until it is complete")
	return cmd
}

func (opts *serviceCheckReleaseOpts) RunE(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if opts.releaseID == "" {
		return fmt.Errorf("-r, --release-id is required")
	}

	if !opts.follow {
		job, err := opts.Fluxd.GetRelease(noInstanceID, flux.ReleaseID(opts.releaseID))
		if err != nil {
			return err
		}
		buf, err := json.MarshalIndent(job, "", "    ")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(buf)
		return err
	}

	var (
		w   = uilive.New()
		job flux.ReleaseJob
		err error
	)
	w.Start()
	for range time.Tick(250 * time.Millisecond) {
		job, err = opts.Fluxd.GetRelease(noInstanceID, flux.ReleaseID(opts.releaseID))
		if err != nil {
			fmt.Fprintf(w, "Status: error querying release.\n") // error will get printed below
			break
		}
		status := "Waiting for job to be claimed..."
		if job.Status != "" {
			status = job.Status
		}
		if delta := time.Since(job.Heartbeat); !job.Heartbeat.IsZero() && delta > largestHeartbeatDelta {
			status = status + fmt.Sprintf(" (warning: no heartbeat in %s, worker may have crashed)", delta)
		}
		fmt.Fprintf(w, "Status: %s\n", status)
		if job.IsFinished() {
			break
		}
	}
	w.Stop()

	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "\n")
	if !job.Success {
		fmt.Fprintf(os.Stdout, "Here's as far as we got:\n")
	} else if job.Spec.Kind == flux.ReleaseKindPlan {
		fmt.Fprintf(os.Stdout, "Here's the plan:\n")
	} else {
		fmt.Fprintf(os.Stdout, "Here's what happened:\n")
	}
	for i, msg := range job.Log {
		fmt.Fprintf(os.Stdout, " %d) %s\n", i+1, msg)
	}

	if job.Spec.Kind == flux.ReleaseKindExecute {
		fmt.Fprintf(os.Stdout, "Took %s\n", time.Since(job.Submitted))
	}
	return nil
}
