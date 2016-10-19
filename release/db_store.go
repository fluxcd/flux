package release

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
)

// DatabaseStore is a job store backed by a sql.DB.
type DatabaseStore struct {
	conn   *sql.DB
	oldest time.Duration
}

var _ flux.ReleaseJobStore = &DatabaseStore{}

// NewDatabaseStore returns a usable DatabaseStore.
// The DB should have a release_jobs table.
func NewDatabaseStore(driver, datasource string, oldest time.Duration) (*DatabaseStore, error) {
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
	var (
		jobStr      string
		claimedAt   nullTime
		heartbeatAt nullTime
	)
	if err := s.conn.QueryRow(`
		SELECT job, claimed_at, heartbeat_at
		  FROM release_jobs
		 WHERE release_id = $1
		   AND instance_id = $2
	`, string(id), string(inst)).Scan(&jobStr, &claimedAt, &heartbeatAt); err == sql.ErrNoRows {
		return flux.ReleaseJob{}, flux.ErrNoSuchReleaseJob
	} else if err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "error getting job")
	}
	var job flux.ReleaseJob
	if err := json.NewDecoder(strings.NewReader(jobStr)).Decode(&job); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling job")
	}
	job.Claimed = time.Time(claimedAt).UTC()
	job.Heartbeat = time.Time(heartbeatAt).UTC()
	return job, nil
}

func (s *DatabaseStore) PutJob(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	job := flux.ReleaseJob{
		Instance:  inst,
		Spec:      spec,
		ID:        flux.NewReleaseID(),
		Submitted: time.Now().UTC(),
	}
	jobBytes, err := json.Marshal(job)
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
	`, string(job.ID), string(job.Instance), job.Submitted, string(jobBytes)); err != nil {
		tx.Rollback()
		return "", errors.Wrap(err, "enqueueing job")
	}

	if err := tx.Commit(); err != nil {
		return "", errors.Wrap(err, "committing insert transaction")
	}
	return job.ID, nil
}

func (s *DatabaseStore) NextJob() (flux.ReleaseJob, error) {
	tx, err := s.conn.Begin()
	if err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "beginning transaction")
	}

	// Have to SELECT submitted_at to ORDER BY it in ql.
	// https://github.com/cznic/ql/issues/138
	var (
		id        string
		jobStr    string
		submitted time.Time
	)
	if err := tx.QueryRow(`
		   SELECT release_id, job, submitted_at
		     FROM release_jobs
		    WHERE claimed_at IS NULL
		 ORDER BY submitted_at DESC
		    LIMIT 1
	`).Scan(&id, &jobStr, &submitted); err == sql.ErrNoRows {
		tx.Commit()
		return flux.ReleaseJob{}, flux.ErrNoReleaseJobAvailable
	} else if err != nil {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Wrap(err, "dequeueing next job")
	}
	var job flux.ReleaseJob
	if err := json.NewDecoder(strings.NewReader(jobStr)).Decode(&job); err != nil {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling job")
	}

	if res, err := tx.Exec(`
		UPDATE release_jobs
		   SET claimed_at = $1
		 WHERE release_id = $2
	`, time.Now().UTC(), id); err != nil {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Wrap(err, "marking job as claimed")
	} else if n, err := res.RowsAffected(); err != nil {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Wrap(err, "after update, checking affected rows")
	} else if n != 1 {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Errorf("wanted to affect 1 row; affected %d", n)
	}

	if err := tx.Commit(); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "committing transaction")
	}
	return job, nil
}

func (s *DatabaseStore) UpdateJob(job flux.ReleaseJob) error {
	jobBytes, err := json.Marshal(job)
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
	`, string(jobBytes), string(job.ID)); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "updating job in database")
	} else if n, err := res.RowsAffected(); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "after update, checking affected rows")
	} else if n == 0 {
		tx.Rollback()
		return flux.ErrNoSuchReleaseJob
	} else if n > 1 {
		tx.Rollback()
		return errors.Errorf("updating job affected %d rows; wanted 1", n)
	}

	if job.IsFinished() {
		if res, err := tx.Exec(`
			UPDATE release_jobs
			   SET finished_at = $1
			 WHERE release_id = $2
		`, job.Finished, string(job.ID)); err != nil {
			tx.Rollback()
			return errors.Wrap(err, "marking finished in database")
		} else if n, err := res.RowsAffected(); err != nil {
			tx.Rollback()
			return errors.Wrap(err, "after marking finished, checking affected rows")
		} else if n != 1 {
			tx.Rollback()
			return errors.Errorf("marking finish wanted to affect 1 row; affected %d", n)
		}
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "committing update transaction")
	}
	return nil
}

func (s *DatabaseStore) Heartbeat(id flux.ReleaseID) error {
	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning heartbeat transaction")
	}

	if res, err := tx.Exec(`
		UPDATE release_jobs
		   SET heartbeat_at = $1
		 WHERE release_id = $2
	`, time.Now().UTC(), string(id)); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "heartbeating job in database")
	} else if n, err := res.RowsAffected(); err != nil {
		tx.Rollback()
		return errors.Wrap(err, "after heartbeat, checking affected rows")
	} else if n == 0 {
		tx.Rollback()
		return flux.ErrNoSuchReleaseJob
	} else if n > 1 {
		tx.Rollback()
		return errors.Errorf("heartbeating job affected %d rows; wanted 1", n)
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "committing heartbeat transaction")
	}
	return nil
}

func (s *DatabaseStore) GC() error {
	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning GC transaction")
	}

	if _, err := tx.Exec(`
		DELETE FROM release_jobs
		      WHERE finished_at IS NOT NULL
		        AND submitted_at < $1
	`, time.Now().UTC().Add(-s.oldest)); err != nil {
		tx.Rollback()
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

type nullTime time.Time

func (t *nullTime) Scan(src interface{}) error {
	switch x := src.(type) {
	case nil:
		return nil
	case time.Time:
		*t = nullTime(x)
		return nil
	}
	return fmt.Errorf("unsupported scan of %s to nullTime", reflect.TypeOf(src))
}
