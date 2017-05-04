package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/history"
	"github.com/weaveworks/flux/job"
)

var ErrTimeout = errors.New("timeout")

// await polls for a job to complete, then for it's commit to be applied
func await(stdout io.Writer, client api.ClientService, jobID job.ID, apply bool) error {
	fmt.Fprintf(stdout, "Job queued\n")
	result, err := awaitJob(client, jobID)
	if err != nil {
		return err
	}
	if result.Revision == "" {
		fmt.Fprintf(stdout, "Nothing to do\n")
		return nil
	}
	fmt.Fprintf(stdout, "Commit pushed: %s\n", result.ShortRevision())

	if apply {
		if err := awaitSync(client, result.Revision); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "Applied\n")
	}

	// FIXME Write out the result here
	return nil
}

// await polls for a job to have been completed, with exponential backoff.
func awaitJob(client api.ClientService, jobID job.ID) (history.CommitEventMetadata, error) {
	var result history.CommitEventMetadata
	err := backoff(100*time.Millisecond, 2, 100, 1*time.Minute, func() (bool, error) {
		j, err := client.JobStatus(noInstanceID, jobID)
		if err != nil {
			return false, err
		}
		switch j.StatusString {
		case job.StatusFailed:
			return false, j.Error
		case job.StatusSucceeded:
			if j.Error != nil {
				// How did we succeed but still get an error!?
				return false, j.Error
			}
			var ok bool
			result, ok = j.Result.(history.CommitEventMetadata)
			if !ok {
				return false, fmt.Errorf("Unknown result type: %T", j.Result)
			}
			return true, nil
		}
		return false, nil
	})
	return result, err
}

// await polls for a commit to have been applied, with exponential backoff.
func awaitSync(client api.ClientService, revision string) error {
	return backoff(100*time.Millisecond, 2, 100, 1*time.Minute, func() (bool, error) {
		refs, err := client.SyncStatus(noInstanceID, revision)
		return err == nil && len(refs) == 0, err
	})
}

// backoff polls for f() to have been completed, with exponential backoff.
func backoff(initialDelay, factor, maxFactor, timeout time.Duration, f func() (bool, error)) error {
	maxDelay := initialDelay * maxFactor
	finish := time.Now().UTC().Add(timeout)
	for delay := initialDelay; time.Now().UTC().Before(finish); delay = min(delay*factor, maxDelay) {
		ok, err := f()
		if ok || err != nil {
			return err
		}
		// If we will have time to try again, sleep
		if time.Now().UTC().Add(delay).Before(finish) {
			time.Sleep(delay)
		}
	}
	return ErrTimeout
}

func min(t1, t2 time.Duration) time.Duration {
	if t1 < t2 {
		return t1
	}
	return t2
}
