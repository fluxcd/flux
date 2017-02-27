package jobs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

// DatabaseStore is a job store backed by a sql.DB.
type DatabaseStore struct {
	conn   dbProxy
	oldest time.Duration
	now    func(dbProxy) (time.Time, error)
}

type dbProxy interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

// NewDatabaseStore returns a usable DatabaseStore.
// The DB should have a jobs table.
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

func (s *DatabaseStore) GetJob(inst flux.InstanceID, id JobID) (Job, error) {
	var (
		job Job
		err error

		// these all need special treatment, either because they can
		// be null, or because they need decoding
		paramsBytes []byte
		claimedAt   nullTime
		heartbeatAt nullTime
		finishedAt  nullTime
		resultBytes []byte
		logBytes    []byte
		done        sql.NullBool
		success     sql.NullBool
		errorBytes  []byte
	)

	if err = s.conn.QueryRow(`
		SELECT queue, method, params, scheduled_at, priority, key, submitted_at, claimed_at, heartbeat_at, finished_at, result, log, status, done, success, error
		  FROM jobs
		 WHERE id = $1
		   AND instance_id = $2
	`, string(id), string(inst)).Scan(
		&job.Queue, &job.Method, &paramsBytes, &job.ScheduledAt, &job.Priority, &job.Key, &job.Submitted,
		&claimedAt, &heartbeatAt, &finishedAt, &resultBytes, &logBytes, &job.Status, &done, &success, &errorBytes,
	); err == sql.ErrNoRows {
		return Job{}, ErrNoSuchJob
	} else if err != nil {
		return Job{}, errors.Wrap(err, "error getting job")
	}

	job.Claimed = claimedAt.Time
	job.Heartbeat = heartbeatAt.Time
	job.Finished = finishedAt.Time
	job.Done = done.Bool
	job.Success = success.Bool

	if job.Params, err = s.scanParams(job.Method, paramsBytes); err != nil {
		return Job{}, errors.Wrap(err, "unmarshaling params")
	}

	if job.Result, err = s.scanResult(job.Method, resultBytes); err != nil {
		return Job{}, errors.Wrap(err, "unmarshaling result")
	}

	var jerr flux.BaseError
	if errorBytes != nil {
		if err = json.Unmarshal(errorBytes, &jerr); err != nil {
			return Job{}, err
		}
		job.Error = &jerr
	}

	if err = json.Unmarshal(logBytes, &job.Log); err != nil {
		return Job{}, errors.Wrap(err, "unmarshaling log")
	}

	return job, nil
}

// PutJobIgnoringDuplicates schedules a job to run. Key field and any
// duplicates are ignored.
func (s *DatabaseStore) PutJobIgnoringDuplicates(inst flux.InstanceID, job Job) (JobID, error) {
	var (
		jobID       = NewJobID()
		status      = "Queued."
		paramsBytes []byte
		err         error
	)
	if job.Queue == "" {
		job.Queue = DefaultQueue
	}
	if job.Params != nil {
		paramsBytes, err = json.Marshal(job.Params)
		if err != nil {
			return JobID(""), errors.Wrap(err, "marshaling params")
		}
	}
	logBytes, err := json.Marshal([]string{status})
	if err != nil {
		return JobID(""), errors.Wrap(err, "marshaling log")
	}

	err = s.Transaction(func(s *DatabaseStore) error {
		now, err := s.now(s.conn)
		if err != nil {
			return errors.Wrap(err, "getting current time")
		}
		if job.ScheduledAt.IsZero() {
			job.ScheduledAt = now
		}
		_, err = s.conn.Exec(`
			INSERT INTO jobs (instance_id, id, queue, method, params, scheduled_at, priority, key, submitted_at, log, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			string(inst),
			string(jobID),
			job.Queue,
			job.Method,
			string(paramsBytes),
			job.ScheduledAt,
			job.Priority,
			job.Key,
			now,
			string(logBytes),
			status,
		)
		return err
	})
	if err != nil {
		return "", err
	}
	return jobID, nil
}

// PutJob schedules a job to run. Users should set the Queue, Method, Params,
// and ScheduledAt fields of the job. If ScheduledAt is nil, the job will run
// immediately. If job Key is not blank, it will be checked for any other
// unfinished duplicate jobs.
func (s *DatabaseStore) PutJob(inst flux.InstanceID, job Job) (JobID, error) {
	var jobID JobID
	err := s.Transaction(func(s *DatabaseStore) (err error) {
		if job.Key != "" {
			var count int
			err = s.conn.QueryRow(`
				SELECT count(1) FROM jobs WHERE instance_id = $1 AND key = $2 AND finished_at IS NULL
			`, string(inst), job.Key).Scan(&count)
			if err != nil {
				return errors.Wrap(err, "looking for existing job")
			}
			if count > 0 {
				return ErrJobAlreadyQueued
			}
		}
		jobID, err = s.PutJobIgnoringDuplicates(inst, job)
		return err
	})
	return jobID, err
}

// Take the next job from specified queues. If queues is nil, all queues are
// used.
func (s *DatabaseStore) NextJob(queues []string) (Job, error) {
	if len(queues) == 0 {
		queues = []string{DefaultQueue}
	}
	var job Job
	err := s.Transaction(func(s *DatabaseStore) error {
		now, err := s.now(s.conn)
		if err != nil {
			return errors.Wrap(err, "getting current time")
		}
		var (
			instanceID  string
			jobID       string
			queue       string
			method      string
			paramsBytes []byte
			scheduledAt time.Time
			priority    int
			key         string
			submittedAt time.Time
			claimedAt   nullTime
			heartbeatAt nullTime
			finishedAt  nullTime
			logStr      string
			status      string
			done        sql.NullBool
			success     sql.NullBool
		)
		query, args, err := sqlx.In(`
			SELECT instance_id, id, queue, method, params,
						 scheduled_at, priority, key, submitted_at,
						 claimed_at, heartbeat_at, finished_at, log, status,
						 done, success
			FROM jobs

			-- Scope it to our selected queues
			WHERE queue IN (?)

			-- Only unclaimed/unfinished jobs are available
			AND claimed_at IS NULL
			AND finished_at IS NULL

			-- Don't make jobs available until after they are scheduled
			AND scheduled_at <= ?

			-- Only one job at a time per instance * queue
			AND instance_id NOT IN (
				SELECT instance_id
				FROM jobs
				WHERE queue IN (?)
				AND claimed_at IS NOT NULL
				AND finished_at IS NULL
				GROUP BY instance_id
			)

			-- subtraction is to work around for ql, not being able to sort
			-- multiple columns in different ways.
			ORDER BY (-1 * priority), scheduled_at, submitted_at
			LIMIT 1`,
			queues,
			now,
			queues,
		)
		if err != nil {
			return errors.Wrap(err, "dequeueing next job")
		}
		query = sqlx.Rebind(sqlx.DOLLAR, query)
		if err := s.conn.QueryRow(query, args...).Scan(
			&instanceID,
			&jobID,
			&queue,
			&method,
			&paramsBytes,
			&scheduledAt,
			&priority,
			&key,
			&submittedAt,
			&claimedAt,
			&heartbeatAt,
			&finishedAt,
			&logStr,
			&status,
			&done,
			&success,
		); err == sql.ErrNoRows {
			return ErrNoJobAvailable
		} else if err != nil {
			return errors.Wrap(err, "dequeueing next job")
		}

		params, err := s.scanParams(method, paramsBytes)
		if err != nil {
			return errors.Wrap(err, "unmarshaling params")
		}

		// NB because we're getting a fresh job, we don't expect any
		// result to be present.

		var log []string
		if err := json.NewDecoder(strings.NewReader(logStr)).Decode(&log); err != nil {
			return errors.Wrap(err, "unmarshaling log")
		}

		job = Job{
			Instance:    flux.InstanceID(instanceID),
			ID:          JobID(jobID),
			Queue:       queue,
			Method:      method,
			Params:      params,
			ScheduledAt: scheduledAt,
			Priority:    priority,
			Key:         key,
			Submitted:   submittedAt,
			Claimed:     claimedAt.Time,
			Heartbeat:   heartbeatAt.Time,
			Finished:    finishedAt.Time,
			Log:         log,
			Status:      status,
			Done:        done.Bool,
			Success:     success.Bool,
		}

		if res, err := s.conn.Exec(`
			UPDATE jobs
				 SET claimed_at = $1
			 WHERE id = $2
				 AND instance_id = $3
		`, now, jobID, instanceID); err != nil {
			return errors.Wrap(err, "marking job as claimed")
		} else if n, err := res.RowsAffected(); err != nil {
			return errors.Wrap(err, "after update, checking affected rows")
		} else if n != 1 {
			return errors.Errorf("wanted to affect 1 row; affected %d", n)
		}
		return nil
	})
	return job, err
}

func (s *DatabaseStore) scanParams(method string, params []byte) (interface{}, error) {
	switch method {
	case ReleaseJob:
		var p ReleaseJobParams
		if params == nil {
			return p, nil
		}
		err := json.Unmarshal(params, &p)
		return p, err
	case AutomatedInstanceJob:
		var p AutomatedInstanceJobParams
		if params == nil {
			return p, nil
		}
		err := json.Unmarshal(params, &p)
		return p, err
	default:
		return nil, ErrUnknownJobMethod
	}
}

func (s *DatabaseStore) scanResult(method string, result []byte) (interface{}, error) {
	switch method {
	case ReleaseJob:
		var r flux.ReleaseResult
		if result == nil {
			return r, nil
		}
		err := json.Unmarshal(result, &r)
		return r, err
	case AutomatedInstanceJob:
		// A result is not expected for these jobs
		return nil, ErrNoResultExpected
	default:
		return nil, ErrUnknownJobMethod
	}
}

func (s *DatabaseStore) UpdateJob(job Job) error {
	paramsBytes, err := json.Marshal(job.Params)
	if err != nil {
		return errors.Wrap(err, "marshaling params")
	}
	resultBytes, err := json.Marshal(job.Result)
	if err != nil {
		return errors.Wrap(err, "marshaling results")
	}
	logBytes, err := json.Marshal(job.Log)
	if err != nil {
		return errors.Wrap(err, "marshaling log")
	}
	errBytes, err := json.Marshal(job.Error)
	if err != nil {
		return errors.Wrap(err, "marshaling error")
	}

	return s.Transaction(func(s *DatabaseStore) error {
		if res, err := s.conn.Exec(`
			UPDATE jobs
				 SET params = $1, result = $2, log = $3, status = $4, error = $5
			 WHERE id = $6
				 AND instance_id = $7
		`, string(paramsBytes), string(resultBytes), string(logBytes), job.Status, string(errBytes), string(job.ID), string(job.Instance)); err != nil {
			return errors.Wrap(err, "updating job in database")
		} else if n, err := res.RowsAffected(); err != nil {
			return errors.Wrap(err, "after update, checking affected rows")
		} else if n == 0 {
			return ErrNoSuchJob
		} else if n > 1 {
			return errors.Errorf("updating job affected %d rows; wanted 1", n)
		}

		if job.Done {
			now, err := s.now(s.conn)
			if err != nil {
				return errors.Wrap(err, "getting current time")
			}
			if res, err := s.conn.Exec(`
				UPDATE jobs
					 SET finished_at = $1, done = $2, success = $3
				 WHERE id = $4
					 AND instance_id = $5
			`, now, job.Done, job.Success, string(job.ID), string(job.Instance)); err != nil {
				return errors.Wrap(err, "marking finished in database")
			} else if n, err := res.RowsAffected(); err != nil {
				return errors.Wrap(err, "after marking finished, checking affected rows")
			} else if n != 1 {
				return errors.Errorf("marking finish wanted to affect 1 row; affected %d", n)
			}
		}
		return nil
	})
}

func (s *DatabaseStore) Heartbeat(id JobID) error {
	return s.Transaction(func(s *DatabaseStore) error {
		now, err := s.now(s.conn)
		if err != nil {
			return errors.Wrap(err, "getting current time")
		}
		if res, err := s.conn.Exec(`
			UPDATE jobs
			SET heartbeat_at = $1
			WHERE id = $2
		`, now, string(id)); err != nil {
			return errors.Wrap(err, "heartbeating job in database")
		} else if n, err := res.RowsAffected(); err != nil {
			return errors.Wrap(err, "after heartbeat, checking affected rows")
		} else if n == 0 {
			return ErrNoSuchJob
		} else if n > 1 {
			return errors.Errorf("heartbeating job affected %d rows; wanted 1", n)
		}
		return nil
	})
}

func (s *DatabaseStore) GC() error {
	// Take current time from the DB. Use the helper function to accommodate
	// for non-portable time functions/queries across different DBs :(
	return s.Transaction(func(s *DatabaseStore) error {
		now, err := s.now(s.conn)
		if err != nil {
			return errors.Wrap(err, "getting current time")
		}

		if _, err := s.conn.Exec(`
			DELETE FROM jobs
						WHERE (finished_at IS NOT NULL AND submitted_at < $1)
						   OR (claimed_at IS NOT NULL
							 AND claimed_at < $1
							 AND (heartbeat_at IS NULL OR heartbeat_at < $1))
		`, now.Add(-s.oldest)); err != nil {
			return errors.Wrap(err, "deleting old jobs")
		}
		return nil
	})
}

func (s *DatabaseStore) sanityCheck() error {
	_, err := s.conn.Query(`SELECT id FROM jobs LIMIT 1`)
	if err != nil {
		return errors.Wrap(err, "failed sanity check for jobs table")
	}
	return nil
}

func (s *DatabaseStore) Transaction(f func(*DatabaseStore) error) error {
	if _, ok := s.conn.(*sql.Tx); ok {
		// Already in a nested transaction
		return f(s)
	}

	tx, err := s.conn.(*sql.DB).Begin()
	if err != nil {
		return err
	}
	err = f(&DatabaseStore{
		conn:   tx,
		oldest: s.oldest,
		now:    s.now,
	})
	if err != nil {
		// Rollback error is ignored as we already have an error in progress
		tx.Rollback()
		return err
	}
	return tx.Commit()
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

func nowFor(driver string) func(dbProxy) (time.Time, error) {
	switch driver {
	case "ql", "ql-mem":
		return func(conn dbProxy) (t time.Time, err error) {
			return t, conn.QueryRow("SELECT now() FROM __Table LIMIT 1").Scan(&t)
		}
	default:
		return func(conn dbProxy) (t time.Time, err error) {
			return t, conn.QueryRow("SELECT now()").Scan(&t)
		}
	}
}
