package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/fluxcd/flux/pkg/api"
	"github.com/fluxcd/flux/pkg/job"
	"github.com/fluxcd/flux/pkg/update"
)

var ErrTimeout = errors.New("timeout")

// await polls for a job to complete, then for the resulting commit to
// be applied
func await(ctx context.Context, stdout, stderr io.Writer, client api.Server, jobID job.ID, apply bool, verbosity int, timeout time.Duration) error {
	result, err := awaitJob(ctx, client, jobID, timeout)
	if err != nil {
		if err == ErrTimeout {
			fmt.Fprintln(stderr, `
We timed out waiting for the result of the operation. This does not
necessarily mean it has failed. You can check the state of the
cluster, or commit logs, to see if there was a result. In general, it
is safe to retry operations.`)
			// because the outcome is unknown, still return the err to indicate an exceptional exit
		}
		return err
	}
	if result.Result != nil {
		update.PrintResults(stdout, result.Result, verbosity)
	}
	if result.Revision != "" {
		fmt.Fprintf(stderr, "Commit pushed:\t%s\n", result.Revision[:7])
	}
	if result.Result == nil {
		fmt.Fprintf(stderr, "Nothing to do\n")
		return nil
	}

	if apply && result.Revision != "" {
		if err := awaitSync(ctx, client, result.Revision, timeout); err != nil {
			if err == ErrTimeout {
				fmt.Fprintln(stderr, `
The operation succeeded, but we timed out waiting for the commit to be
applied. This does not necessarily mean there is a problem. Use

    fluxctl sync

to run a sync interactively.`)
				return nil
			}
			return err
		}
		fmt.Fprintf(stderr, "Commit applied:\t%s\n", result.Revision[:7])
	}

	return nil
}

// await polls for a job to have been completed, with exponential backoff.
func awaitJob(ctx context.Context, client api.Server, jobID job.ID, timeout time.Duration) (job.Result, error) {
	var result job.Result
	err := backoff(100*time.Millisecond, 2, 50, timeout, func() (bool, error) {
		j, err := client.JobStatus(ctx, jobID)
		if err != nil {
			return false, err
		}
		switch j.StatusString {
		case job.StatusFailed:
			result = j.Result
			return false, j
		case job.StatusSucceeded:
			if j.Err != "" {
				// How did we succeed but still get an error!?
				return false, j
			}
			result = j.Result
			return true, nil
		}
		return false, nil
	})
	return result, err
}

// await polls for a commit to have been applied, with exponential backoff.
func awaitSync(ctx context.Context, client api.Server, revision string, timeout time.Duration) error {
	return backoff(1*time.Second, 2, 10, timeout, func() (bool, error) {
		refs, err := client.SyncStatus(ctx, revision)
		return err == nil && len(refs) == 0, err
	})
}

// backoff polls for f() to have been completed, with exponential backoff.
func backoff(initialDelay, factor, maxFactor, timeout time.Duration, f func() (bool, error)) error {
	maxDelay := initialDelay * maxFactor
	finish := time.Now().Add(timeout)
	for delay := initialDelay; time.Now().Before(finish); delay = min(delay*factor, maxDelay) {
		ok, err := f()
		if ok || err != nil {
			return err
		}
		// If we don't have time to try again, stop
		if time.Now().Add(delay).After(finish) {
			break
		}
		time.Sleep(delay)
	}
	return ErrTimeout
}

func min(t1, t2 time.Duration) time.Duration {
	if t1 < t2 {
		return t1
	}
	return t2
}
