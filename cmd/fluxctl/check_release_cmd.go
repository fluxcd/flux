package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gosuri/uilive"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/jobs"
)

const largestHeartbeatDelta = 5 * time.Second
const retryTimeout = 2 * time.Minute

type serviceCheckReleaseOpts struct {
	*serviceOpts
	releaseID string
	noFollow  bool
	noTty     bool
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
	cmd.Flags().BoolVar(&opts.noFollow, "no-follow", false, "dump release job as JSON to stdout")
	cmd.Flags().BoolVar(&opts.noTty, "no-tty", false, "forces simpler, non-TTY status output")
	return cmd
}

func (opts *serviceCheckReleaseOpts) RunE(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errorWantedNoArgs
	}

	if opts.releaseID == "" {
		return fmt.Errorf("-r, --release-id is required")
	}

	if opts.noFollow {
		job, err := opts.API.GetRelease(noInstanceID, jobs.JobID(opts.releaseID))
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
		w    io.Writer = os.Stdout
		stop           = func() {}
	)
	if !opts.noTty && isatty.IsTerminal(os.Stdout.Fd()) {
		liveWriter := uilive.New()
		liveWriter.Start()
		var stopOnce sync.Once
		w, stop = liveWriter, func() { stopOnce.Do(liveWriter.Stop) }
	}
	var (
		job jobs.Job
		err error

		prevStatus            string
		lastHeartbeatDatabase time.Time
		lastHeartbeatLocal    = time.Now()

		retryCount    = 0
		lastSucceeded = time.Now()
	)

	for range time.Tick(time.Second) {
		if retryCount > 0 {
			fmt.Fprintf(w, "Last status (%s): %s\n", lastSucceeded.Format(time.Kitchen), prevStatus)
			fmt.Fprintf(w, "Service unavailable. Retrying (#%d) ...\n", retryCount)
		}

		job, err = opts.API.GetRelease(noInstanceID, jobs.JobID(opts.releaseID))
		if err != nil {
			if err, ok := errors.Cause(err).(*transport.APIError); ok && err.IsUnavailable() {
				if time.Since(lastSucceeded) > retryTimeout {
					stop()
					fmt.Fprintln(os.Stdout, "Giving up; you can try again with")
					fmt.Fprintf(os.Stdout, "    fluxctl check-release -r %s\n", opts.releaseID)
					fmt.Fprintln(os.Stdout)
					break
				}
				retryCount++
				continue
			}
			fmt.Fprintf(w, "Status: error querying release.\n") // error will get printed below
			break
		}

		lastSucceeded = time.Now()
		retryCount = 0
		status := "Waiting for job to be claimed..."
		if job.Status != "" {
			status = job.Status
		}

		// Checking heartbeat is a bit tricky. We get a timestamp in database
		// time, which may be radically different to our time. I've chosen to
		// check liveness by marking local time whenever the heartbeat time
		// changes. Going long enough without updating that local timestamp
		// triggers the warning.
		if !job.Claimed.IsZero() {
			if job.Heartbeat != lastHeartbeatDatabase {
				lastHeartbeatDatabase = job.Heartbeat
				lastHeartbeatLocal = time.Now()
			}
			if delta := time.Since(lastHeartbeatLocal); delta > largestHeartbeatDelta {
				status += fmt.Sprintf(" -- no heartbeat in %s; worker may have crashed", delta)
			}
		}

		if status != prevStatus {
			fmt.Fprintf(w, "Status: %s\n", status)
		}
		prevStatus = status

		if job.Done {
			break
		}
	}
	stop()

	if err != nil {
		return err
	}

	spec := job.Params.(jobs.ReleaseJobParams)

	fmt.Fprintf(os.Stdout, "\n")
	if !job.Success {
		fmt.Fprintf(os.Stdout, "Here's as far as we got:\n")
	} else if spec.Kind == flux.ReleaseKindPlan {
		fmt.Fprintf(os.Stdout, "Here's the plan:\n")
	} else {
		fmt.Fprintf(os.Stdout, "Here's what happened:\n")
	}
	for i, msg := range job.Log {
		fmt.Fprintf(os.Stdout, " %d) %s\n", i+1, msg)
	}

	if spec.Kind == flux.ReleaseKindExecute {
		fmt.Fprintf(os.Stdout, "Took %s\n", job.Finished.Sub(job.Submitted))
	}
	return nil
}
