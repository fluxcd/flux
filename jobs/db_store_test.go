package jobs

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/db"
)

var (
	databaseSource = flag.String("database-source", "", `Database source name. The default is a temporary DB using ql`)

	done        chan error
	errRollback = fmt.Errorf("Rolling back test data")
)

func mkDBFile(t *testing.T) string {
	f, err := ioutil.TempFile("", "fluxy-testdb")
	if err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func bailIfErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

// Setup sets up stuff for testing, creating a new database
func Setup(t *testing.T) *DatabaseStore {
	if *databaseSource == "" {
		*databaseSource = "file://" + mkDBFile(t)
	}

	u, err := url.Parse(*databaseSource)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.Migrate(*databaseSource, "../db/migrations"); err != nil {
		t.Fatal(err)
	}

	db, err := NewDatabaseStore(db.DriverForScheme(u.Scheme), *databaseSource, 1*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	newDB := make(chan *DatabaseStore)
	done = make(chan error)
	go func() {
		done <- db.Transaction(func(tx *DatabaseStore) error {
			// Pass out the tx so we can run the test
			newDB <- tx
			// Wait for the test to finish
			return <-done
		})
	}()
	// Get the new database
	return <-newDB
}

// Cleanup cleans up after a test
func Cleanup(t *testing.T, database *DatabaseStore) {
	if done != nil {
		done <- errRollback
		err := <-done
		if err != errRollback {
			t.Fatalf("Unexpected error %q", err)
		}
		done = nil
	}
}

func TestDatabaseStore(t *testing.T) {
	instance := flux.InstanceID("instance")
	db := Setup(t)
	defer Cleanup(t, db)

	// Get a job when there are none
	_, err := db.NextJob(nil)
	if err != ErrNoJobAvailable {
		t.Fatalf("Expected ErrNoJobAvailable, got %q", err)
	}

	// Put some jobs
	backgroundJobID, err := db.PutJob(instance, Job{
		Method:   ReleaseJob,
		Params:   ReleaseJobParams{},
		Priority: PriorityBackground,
	})
	bailIfErr(t, err)
	interactiveJobID, err := db.PutJob(instance, Job{
		Key:      "2",
		Method:   ReleaseJob,
		Params:   ReleaseJobParams{},
		Priority: PriorityInteractive,
	})
	bailIfErr(t, err)

	// Put a duplicate
	duplicateID, err := db.PutJob(instance, Job{
		Key:      "2",
		Method:   ReleaseJob,
		Params:   ReleaseJobParams{},
		Priority: PriorityInteractive,
	})
	if err != ErrJobAlreadyQueued {
		t.Errorf("Expected duplicate job to return ErrJobAlreadyQueued, got: %q", err)
	}
	if string(duplicateID) != "" {
		t.Errorf("Expected no id for duplicate job, got: %q", duplicateID)
	}

	// Take one
	interactiveJob, err := db.NextJob(nil)
	bailIfErr(t, err)
	// - It should be the highest priority
	if interactiveJob.ID != interactiveJobID {
		t.Errorf("Got a lower priority job when a higher one was available")
	}
	// - It should have a default queue
	if interactiveJob.Queue != DefaultQueue {
		t.Errorf("job default queue (%q) was not expected (%q)", interactiveJob.Queue, DefaultQueue)
	}
	// - It should have been scheduled in the past
	now, err := db.now(db.conn)
	bailIfErr(t, err)
	if interactiveJob.ScheduledAt.IsZero() || interactiveJob.ScheduledAt.After(now) {
		t.Errorf("expected job to be scheduled in the past")
	}
	// - It should have a log and status
	if len(interactiveJob.Log) == 0 || interactiveJob.Status == "" {
		t.Errorf("expected job to have a log and status")
	}

	// Put a duplicate (when existing is claimed)
	// - It should succeed
	_, err = db.PutJob(instance, Job{
		Key:      "2",
		Method:   ReleaseJob,
		Params:   ReleaseJobParams{},
		Priority: 1, // low priority, so it won't interfere with other jobs
	})
	bailIfErr(t, err)

	// Update the job
	newStatus := "Being used in testing"
	interactiveJob.Status = newStatus
	interactiveJob.Log = append(interactiveJob.Log, newStatus)
	bailIfErr(t, db.UpdateJob(interactiveJob))
	// - It should have saved the changes
	interactiveJob, err = db.GetJob(instance, interactiveJobID)
	bailIfErr(t, err)
	if interactiveJob.Status != newStatus || len(interactiveJob.Log) != 2 || interactiveJob.Log[1] != interactiveJob.Status {
		t.Errorf("expected job to have new log and status")
	}

	// Heartbeat the job
	oldHeartbeat := interactiveJob.Heartbeat
	bailIfErr(t, db.Heartbeat(interactiveJobID))
	// - Heartbeat time should be updated
	interactiveJob, err = db.GetJob(instance, interactiveJobID)
	bailIfErr(t, err)
	if !interactiveJob.Heartbeat.After(oldHeartbeat) {
		t.Errorf("expected job heartbeat to have been updated")
	}

	// Take the next
	backgroundJob, err := db.NextJob(nil)
	bailIfErr(t, err)
	// - It should be different
	if backgroundJob.ID != backgroundJobID {
		t.Errorf("Got a different job than expected")
	}

	// Finish one
	backgroundJob.Done = true
	backgroundJob.Success = true
	bailIfErr(t, db.UpdateJob(backgroundJob))
	// - Status should be changed
	backgroundJob, err = db.GetJob(instance, backgroundJobID)
	bailIfErr(t, err)
	if !backgroundJob.Done || !backgroundJob.Success {
		t.Errorf("expected job to have been marked as done")
	}

	// GC
	// - Advance time so we can gc stuff
	db.now = func(_ dbProxy) (time.Time, error) {
		return time.Now().Add(2 * time.Minute), nil
	}
	bailIfErr(t, db.GC())
	// - Finished should be removed
	_, err = db.GetJob(instance, backgroundJobID)
	if err != ErrNoSuchJob {
		t.Errorf("expected ErrNoSuchJob, got %q", err)
	}
}

func TestDatabaseStoreScheduledJobs(t *testing.T) {
	instance := flux.InstanceID("instance")
	now := time.Now()

	for _, example := range []struct {
		name          string        // name of this test
		jobs          []Job         // Jobs to put in the queue
		offset        time.Duration // Amount to travel forward
		expectedIndex int           // Index of expected job
	}{
		{
			"basics",
			[]Job{
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
				},
			},
			2 * time.Minute,
			0,
		},
		{
			"higher priorities",
			[]Job{
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
					Priority:    1,
				},
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
					Priority:    10,
				},
			},
			2 * time.Minute,
			1,
		},
		{
			"scheduled first",
			[]Job{
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
				},
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(5 * time.Second),
				},
			},
			2 * time.Minute,
			1,
		},
		{
			"submitted first",
			[]Job{
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
				},
				{
					Method:      ReleaseJob,
					Params:      ReleaseJobParams{},
					ScheduledAt: now.Add(1 * time.Minute),
				},
			},
			2 * time.Minute,
			0,
		},
	} {
		db := Setup(t)

		// Stub now so we can time-travel
		db.now = func(_ dbProxy) (time.Time, error) {
			return now, nil
		}

		// Put some scheduled jobs
		var ids []JobID
		for i, job := range example.jobs {
			id, err := db.PutJob(instance, job)
			if err != nil {
				t.Errorf("[%s] putting job onto queue: %v", example.name, err)
			} else {
				ids = append(ids, id)
			}

			// Advance time so each job has a different submission timestamp
			db.now = func(_ dbProxy) (time.Time, error) {
				return now.Add(time.Duration(i) * time.Second), nil
			}
		}

		// Check nothing is available
		if _, err := db.NextJob(nil); err != ErrNoJobAvailable {
			t.Fatalf("[%s] Expected ErrNoJobAvailable, got %q", example.name, err)
		}

		// Advance time so it is available
		db.now = func(_ dbProxy) (time.Time, error) {
			return now.Add(example.offset), nil
		}

		// It should be available
		job, err := db.NextJob(nil)
		if err != nil {
			t.Errorf("[%s] getting job from queue: %v", example.name, err)
			continue
		}
		if job.ID != ids[example.expectedIndex] {
			t.Fatalf("[%s] Expected scheduled job, got %q", example.name, job.ID)
		}

		Cleanup(t, db)
	}
}
