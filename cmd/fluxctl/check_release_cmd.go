package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gosuri/uilive"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/weaveworks/flux"
	transport "github.com/weaveworks/flux/http"
)

const largestHeartbeatDelta = 5 * time.Second

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
		job, err := opts.API.GetRelease(noInstanceID, flux.ReleaseID(opts.releaseID))
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
		w, stop = liveWriter, liveWriter.Stop
	}
	var (
		job flux.ReleaseJob
		err error

		prevStatus            string
		lastHeartbeatDatabase time.Time
		lastHeartbeatLocal    = time.Now()
		initialRetryBackoff   = 4
		abandonRetryThreshold = 60
		retryBackoff          = initialRetryBackoff
		retryNext             = 0
	)

	for range time.Tick(time.Second) {
		if retryNext > 0 {
			fmt.Fprintf(w, "Last status: %s\n", prevStatus)
			fmt.Fprintf(w, "Service unavailable. Retrying in %d sec.\n", retryNext)
			retryNext--
			continue
		}

		job, err = opts.API.GetRelease(noInstanceID, flux.ReleaseID(opts.releaseID))
		if err != nil {
			if err, ok := errors.Cause(err).(*transport.APIError); ok && err.IsUnavailable() {
				if retryBackoff > abandonRetryThreshold {
					stop()
					fmt.Fprintln(os.Stdout, "Giving up retrying for now. You can check again later with")
					fmt.Fprintf(os.Stdout, "    %s -r %s\n", cmd.CommandPath(), opts.releaseID)
					return errors.New("Abandoned after too many retries.")
				}
				retryNext = retryBackoff
				retryBackoff *= 2
				continue
			}
			fmt.Fprintf(w, "Status: error querying release.\n") // error will get printed below
			break
		}

		retryBackoff = initialRetryBackoff
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
		fmt.Fprintf(os.Stdout, "Took %s\n", job.Finished.Sub(job.Submitted))
	}
	return nil
}
