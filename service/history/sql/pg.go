package sql

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/service"
)

// A history DB that uses a postgres database
type pgDB struct {
	*DB
}

func (db *pgDB) eventsQuery() squirrel.SelectBuilder {
	return db.Select(
		"id", "service_ids", "type", "started_at", "ended_at", "log_level",
		"message", "metadata",
	).
		From("events").
		OrderBy("started_at desc")
}

func (db *pgDB) scanEvents(query squirrel.Sqlizer) ([]event.Event, error) {
	rows, err := squirrel.QueryWith(db, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []event.Event
	for rows.Next() {
		var (
			h             event.Event
			serviceIDs    pq.StringArray
			metadataBytes []byte
		)
		if err := rows.Scan(
			&h.ID,
			&serviceIDs,
			&h.Type,
			&h.StartedAt,
			&h.EndedAt,
			&h.LogLevel,
			&h.Message,
			&metadataBytes,
		); err != nil {
			return nil, err
		}
		for _, id := range serviceIDs {
			h.ServiceIDs = append(h.ServiceIDs, flux.MustParseResourceID(id))
		}

		if len(metadataBytes) > 0 {
			switch h.Type {
			case event.EventCommit:
				var m event.CommitEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = &m
			case event.EventSync:
				var m event.SyncEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = &m
			case event.EventRelease:
				var m event.ReleaseEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = &m
			case event.EventAutoRelease:
				var m event.AutoReleaseEventMetadata
				if err := json.Unmarshal(metadataBytes, &m); err != nil {
					return nil, err
				}
				h.Metadata = &m
			}
		}
		events = append(events, h)
	}
	return events, rows.Err()
}

func (db *pgDB) EventsForService(inst service.InstanceID, service flux.ResourceID, before time.Time, limit int64, after time.Time) ([]event.Event, error) {
	q := db.eventsQuery().
		Where("instance_id = ?", string(inst)).
		Where("service_ids @> ?", pq.StringArray{service.String()}).
		Where("started_at < ?", before).
		Where("started_at > ?", after)
	if limit >= 0 {
		q = q.Limit(uint64(limit))
	}
	return db.scanEvents(q)
}

func (db *pgDB) AllEvents(inst service.InstanceID, before time.Time, limit int64, after time.Time) ([]event.Event, error) {
	q := db.eventsQuery().
		Where("instance_id = ?", string(inst)).
		Where("started_at < ?", before).
		Where("started_at > ?", after)
	if limit >= 0 {
		q = q.Limit(uint64(limit))
	}
	return db.scanEvents(q)
}

func (db *pgDB) GetEvent(id event.EventID) (event.Event, error) {
	es, err := db.scanEvents(db.eventsQuery().Where("id = ?", string(id)))
	if err != nil {
		return event.Event{}, err
	}
	if len(es) <= 0 {
		return event.Event{}, fmt.Errorf("event not found")
	}
	return es[0], nil
}

func (db *pgDB) LogEvent(inst service.InstanceID, e event.Event) error {
	j, err := json.Marshal(e.Metadata)
	if err != nil {
		return err
	}
	startedAt := e.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	serviceIDs := pq.StringArray{}
	for _, id := range e.ServiceIDs {
		serviceIDs = append(serviceIDs, id.String())
	}
	_, err = db.driver.Exec(
		`INSERT INTO events
		(instance_id, service_ids, type, log_level, metadata, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		string(inst),
		serviceIDs,
		e.Type,
		e.LogLevel,
		j,
		startedAt,
		pq.NullTime{Time: e.EndedAt.UTC(), Valid: !e.EndedAt.IsZero()},
	)
	return err
}

func (db *pgDB) sanityCheck() (err error) {
	_, err = db.driver.Query("SELECT instance_id, id, message, started_at FROM events LIMIT 1")
	if err != nil {
		return errors.Wrap(err, "sanity checking events table")
	}
	return nil
}
