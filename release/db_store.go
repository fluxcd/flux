package release

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	flux "github.com/weaveworks/fluxy"
)

// DatabaseStore is a job store backed by a sql.DB.
type DatabaseStore struct {
	conn   *sql.DB
	oldest time.Duration
	now    func(*sql.DB) (time.Time, error)
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
		now:    nowFor(driver),
	}
	return s, s.sanityCheck()
}

func (s *DatabaseStore) GetJob(inst flux.InstanceID, id flux.ReleaseID) (flux.ReleaseJob, error) {
	var (
		specStr     string
		submittedAt time.Time
		claimedAt   nullTime
		heartbeatAt nullTime
		finishedAt  nullTime
		logStr      string
		status      string
		success     sql.NullBool
	)
	if err := s.conn.QueryRow(`
		SELECT spec, submitted_at, claimed_at, heartbeat_at, finished_at, log, status, success
		  FROM release_jobs
		 WHERE release_id = $1
		   AND instance_id = $2
	`, string(id), string(inst)).Scan(
		&specStr, &submittedAt, &claimedAt, &heartbeatAt, &finishedAt, &logStr, &status, &success,
	); err == sql.ErrNoRows {
		return flux.ReleaseJob{}, flux.ErrNoSuchReleaseJob
	} else if err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "error getting job")
	}

	var spec flux.ReleaseJobSpec
	if err := json.NewDecoder(strings.NewReader(specStr)).Decode(&spec); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling spec")
	}
	var log []string
	if err := json.NewDecoder(strings.NewReader(logStr)).Decode(&log); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling log")
	}

	return flux.ReleaseJob{
		Instance:  inst,
		Spec:      spec,
		ID:        id,
		Submitted: submittedAt,
		Claimed:   claimedAt.Time,
		Heartbeat: heartbeatAt.Time,
		Finished:  finishedAt.Time,
		Log:       log,
		Status:    status,
		Success:   success.Bool,
	}, nil
}

func (s *DatabaseStore) PutJob(inst flux.InstanceID, spec flux.ReleaseJobSpec) (flux.ReleaseID, error) {
	var (
		releaseID = flux.NewReleaseID()
		status    = "Submitted job."
	)
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return flux.ReleaseID(""), errors.Wrap(err, "marshaling spec")
	}
	logBytes, err := json.Marshal([]string{status})
	if err != nil {
		return flux.ReleaseID(""), errors.Wrap(err, "marshaling log")
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return "", errors.Wrap(err, "beginning insert transaction")
	}

	if _, err := tx.Exec(`
		INSERT INTO release_jobs (release_id, instance_id, spec, submitted_at, log, status)
		     VALUES ($1, $2, $3, now(), $4, $5)
	`, string(releaseID), string(inst), string(specBytes), string(logBytes), status); err != nil {
		tx.Rollback()
		return "", errors.Wrap(err, "enqueueing job")
	}

	if err := tx.Commit(); err != nil {
		return "", errors.Wrap(err, "committing insert transaction")
	}
	return releaseID, nil
}

func (s *DatabaseStore) NextJob() (flux.ReleaseJob, error) {
	tx, err := s.conn.Begin()
	if err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "beginning transaction")
	}

	var (
		releaseID   string
		instanceID  string
		specStr     string
		submittedAt time.Time
		claimedAt   nullTime
		heartbeatAt nullTime
		finishedAt  nullTime
		logStr      string
		status      string
		success     sql.NullBool
	)
	if err := tx.QueryRow(`
		   SELECT release_id, instance_id, spec, submitted_at, claimed_at, heartbeat_at, finished_at, log, status, success
		     FROM release_jobs
		    WHERE claimed_at IS NULL
		 ORDER BY submitted_at DESC
		    LIMIT 1
	`).Scan(
		&releaseID, &instanceID, &specStr, &submittedAt, &claimedAt, &heartbeatAt, &finishedAt, &logStr, &status, &success,
	); err == sql.ErrNoRows {
		tx.Commit()
		return flux.ReleaseJob{}, flux.ErrNoReleaseJobAvailable
	} else if err != nil {
		tx.Rollback()
		return flux.ReleaseJob{}, errors.Wrap(err, "dequeueing next job")
	}

	var spec flux.ReleaseJobSpec
	if err := json.NewDecoder(strings.NewReader(specStr)).Decode(&spec); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling spec")
	}
	var log []string
	if err := json.NewDecoder(strings.NewReader(logStr)).Decode(&log); err != nil {
		return flux.ReleaseJob{}, errors.Wrap(err, "unmarshaling log")
	}

	job := flux.ReleaseJob{
		Instance:  flux.InstanceID(instanceID),
		Spec:      spec,
		ID:        flux.ReleaseID(releaseID),
		Submitted: submittedAt,
		Claimed:   claimedAt.Time,
		Heartbeat: heartbeatAt.Time,
		Finished:  finishedAt.Time,
		Log:       log,
		Status:    status,
		Success:   success.Bool,
	}

	if res, err := tx.Exec(`
		UPDATE release_jobs
		   SET claimed_at = now()
		 WHERE release_id = $1
	`, releaseID); err != nil {
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
	specBytes, err := json.Marshal(job.Spec)
	if err != nil {
		return errors.Wrap(err, "marshaling spec")
	}
	logBytes, err := json.Marshal(job.Log)
	if err != nil {
		return errors.Wrap(err, "marshaling log")
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning update transaction")
	}

	if res, err := tx.Exec(`
		UPDATE release_jobs
		   SET spec = $1, log = $2, status = $3
		 WHERE release_id = $4
	`, string(specBytes), string(logBytes), job.Status, string(job.ID)); err != nil {
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
			   SET finished_at = now(), success = $1
			 WHERE release_id = $2
		`, job.Success, string(job.ID)); err != nil {
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
		   SET heartbeat_at = now()
		 WHERE release_id = $1
	`, string(id)); err != nil {
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
	// Take current time from the DB. Use the helper function to accommodate
	// for non-portable time functions/queries across different DBs :(
	now, err := s.now(s.conn)
	if err != nil {
		return errors.Wrap(err, "getting current time")
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return errors.Wrap(err, "beginning GC transaction")
	}

	if _, err := tx.Exec(`
		DELETE FROM release_jobs
		      WHERE finished_at IS NOT NULL
		        AND submitted_at < $1
	`, now.Add(-s.oldest)); err != nil {
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

type nullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

func (n *nullTime) Scan(value interface{}) error {
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	n.Valid = true
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("unsupported Scan of %T into nullTime", value)
	}
	n.Time = t
	return nil
}

func nowFor(driver string) func(*sql.DB) (time.Time, error) {
	switch driver {
	case "ql", "ql-mem":
		return func(conn *sql.DB) (t time.Time, err error) {
			return t, conn.QueryRow("SELECT now() FROM __Table LIMIT 1").Scan(&t)
		}
	default:
		return func(conn *sql.DB) (t time.Time, err error) {
			return t, conn.QueryRow("SELECT now()").Scan(&t)
		}
	}
}
