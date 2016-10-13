package release

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
)

// DatabaseStore is a job store backed by a sql.DB.
type DatabaseStore struct {
	conn   *sql.DB
	oldest time.Duration
}

// NewDatabaseStore returns a usable DatabaseStore.
// The DB should have a job_queue table.
func NewDatabaseStore(driver, datasource string, oldest time.Duration) (*DatabaseStore, error) {
	if oldest < time.Hour {
		return nil, errors.New("oldest must be at least 1 hour")
	}
	conn, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	s := &DatabaseStore{
		conn:   conn,
		oldest: oldest,
	}
	return s, s.sanityCheck()
}

func (s *DatabaseStore) GetJob(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	var job flux.ReleaseJob
	if err := s.conn.QueryRow(`
		SELECT job
		  FROM release_jobs
		 WHERE release_id = $1
		   AND instance_id = $2
	`, string(id), string(inst)).Scan(&job); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "error getting job")
	}
	return job, nil
}

func (s *DatabaseStore) PutJob(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	job := flux.ReleaseJob{
		Instance:  inst,
		Spec:      spec,
		ID:        flux.NewReleaseID(),
		Submitted: time.Now(),
	}
	jobText, err := json.Marshal(job)
	if err != nil {
		return flux.ReleaseID(""), errors.Wrap(err, "marshaling job")
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return "", errors.Wrap(err, "beginning insert transaction")
	}

	if _, err := tx.Exec(`
		INSERT INTO release_jobs (release_id, instance_id, submitted_at, job)
		VALUES ($1, $2, $3, $4)
	`, string(job.ID), string(job.Instance), job.Submitted, string(jobText)); err != nil {
		tx.Rollback()
		return "", errors.Wrap(err, "enqueueing job")
	}

	if err := tx.Commit(); err != nil {
		return "", errors.Wrap(err, "committing insert transaction")
	}
	return job.ID, nil
}

func (s *DatabaseStore) NextJob() (flux.ReleaseJob, error) {
	var job flux.ReleaseJob
	if err := s.conn.QueryRow(`
		   SELECT job
		     FROM release_jobs
		    WHERE claimed_at IS NULL
		 ORDER BY release_jobs.submitted_at DESC
		    LIMIT 1
	`).Scan(&job); err == sql.ErrNoRows {
		return flux.ReleaseJob{}, flux.ErrNoReleaseJobAvailable
	} else if err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "dequeueing next job")
	}
	return job, nil
}

func (s *DatabaseStore) UpdateJob(job flux.ReleaseJob) error {
	jobText, err := json.Marshal(job)
	if err != nil {
		return errors.Wrap(err, "marshaling job")
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning update transaction")
	}

	if res, err := tx.Exec(`
		UPDATE release_jobs
		   SET job = $1
		 WHERE release_id = $2
	`, string(jobText), string(job.ID)); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "updating job in database")
	} else if n, err := res.RowsAffected(); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "after update, checking affected rows")
	} else if n != 1 {
		tx.Rollback()
		return errors.Errorf("wanted to affect 1 row; affected %d", n)
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "committing update transaction")
	}
	return nil
}

func (s *DatabaseStore) GC() error {
	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning GC transaction")
	}

	// This can delete in-progress jobs that take longer than s.oldest.
	// TODO(pb): add finished_at column and check for it?
	if _, err := tx.Exec(`
		DELETE FROM release_jobs
		WHERE submitted_at < $1
	`, time.Now().Add(-s.oldest)); err != nil {
		tx.Rollback() // ignore error
		return errors.Wrap(err, "deleting old jobs")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "committing GC transaction")
	}
	return nil
}

func (s *DatabaseStore) sanityCheck() error {
	_, err := s.conn.Query(`SELECT release_id FROM release_jobs LIMIT 1`)
	if err != nil {
		return errors.Wrap(err, "failed sanity check for release_jobs table")
	}
	return nil
}
