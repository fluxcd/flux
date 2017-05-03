package sql

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/history"
)

// A history DB that uses a ql database
type qlDB struct {
	*DB
}

func (db *qlDB) eventsQuery() squirrel.SelectBuilder {
	return db.Select(
		"id(events)", "type", "started_at", "ended_at", "log_level", "message", "metadata",
	).
		From("events").
		OrderBy("started_at desc")
}

func (db *qlDB) scanEvents(query squirrel.Sqlizer) ([]history.Event, error) {
	rows, err := squirrel.QueryWith(db, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []history.Event
	for rows.Next() {
		var (
			h             history.Event
			metadataBytes []byte
		)
		if err := rows.Scan(
			&h.ID,
			&h.Type,
			&h.StartedAt,
			&h.EndedAt,
			&h.LogLevel,
			&h.Message,
			&metadataBytes,
		); err != nil {
			return nil, err
		}

		if len(metadataBytes) > 0 {
			switch h.Type {
			case history.EventCommit:
				var m history.CommitEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = m
			case history.EventRelease:
				var m history.ReleaseEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = m
			}
		}
		events = append(events, h)
	}
	return events, rows.Err()
}

func (db *qlDB) EventsForService(inst flux.InstanceID, service flux.ServiceID, before time.Time, limit int64) ([]history.Event, error) {
	q := db.eventsQuery().
		Where("instance_id = ?", string(inst)).
		Where("id(e) IN (select event_id from event_service_ids WHERE service_id = ?)", string(service)).
		Where("started_at < ?", before)
	if limit >= 0 {
		q = q.Limit(uint64(limit))
	}
	events, err := db.scanEvents(q)
	if err != nil {
		return nil, err
	}
	return db.loadServiceIDs(events)
}

func (db *qlDB) AllEvents(inst flux.InstanceID, before time.Time, limit int64) ([]history.Event, error) {
	q := db.eventsQuery().
		Where("instance_id = ?", string(inst)).
		Where("started_at < ?", before)
	if limit >= 0 {
		q = q.Limit(uint64(limit))
	}
	events, err := db.scanEvents(q)
	if err != nil {
		return nil, err
	}
	return db.loadServiceIDs(events)
}

func (db *qlDB) GetEvent(id history.EventID) (history.Event, error) {
	es, err := db.scanEvents(db.eventsQuery().Where("id(events) = ?", string(id)))
	if err != nil {
		return history.Event{}, err
	}
	if len(es) <= 0 {
		return history.Event{}, fmt.Errorf("event not found")
	}
	es, err = db.loadServiceIDs(es)
	return es[0], err
}

func (db *qlDB) loadServiceIDs(events []history.Event) ([]history.Event, error) {
	for _, e := range events {
		rows, err := db.driver.Query(`SELECT service_id from event_service_ids where event_id = $1`, e.ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			e.ServiceIDs = append(e.ServiceIDs, flux.ServiceID(id))
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (db *qlDB) LogEvent(inst flux.InstanceID, e history.Event) (err error) {
	metadata, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	startedAt := e.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	tx, err := db.driver.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	result, err := tx.Exec(
		`INSERT INTO events
		(instance_id, type, log_level, metadata, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		string(inst),
		e.Type,
		e.LogLevel,
		string(metadata),
		startedAt,
		pq.NullTime{Time: e.EndedAt.UTC(), Valid: !e.EndedAt.IsZero()},
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	for _, serviceID := range e.ServiceIDs {
		_, err := tx.Exec(
			`INSERT INTO event_service_ids
			(event_id, service_id)
			VALUES ($1, $2)`,
			id, string(serviceID),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *qlDB) sanityCheck() (err error) {
	_, err = db.driver.Query("SELECT instance_id, id(), message, started_at FROM events LIMIT 1")
	if err != nil {
		return errors.Wrap(err, "sanity checking events table")
	}
	return nil
}
