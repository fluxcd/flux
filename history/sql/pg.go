package sql

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
)

// A history DB that uses a postgres database
type pgDB struct {
	*DB
}

func (db *pgDB) queryEvents(query string, params ...interface{}) ([]flux.Event, error) {
	rows, err := db.driver.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []flux.Event
	for rows.Next() {
		var (
			h             flux.Event
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
			h.ServiceIDs = append(h.ServiceIDs, flux.ServiceID(id))
		}

		if len(metadataBytes) > 0 {
			switch h.Type {
			case flux.EventRelease:
				var m flux.ReleaseEventMetadata
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

func (db *pgDB) EventsForService(inst flux.InstanceID, service flux.ServiceID) ([]flux.Event, error) {
	return db.queryEvents(
		`SELECT id, service_ids, type, started_at, ended_at, log_level, message, metadata
		FROM events
		WHERE instance_id = $1
		AND service_ids @> $2
		ORDER BY started_at DESC`,
		string(inst),
		pq.StringArray{string(service)},
	)
}

func (db *pgDB) AllEvents(inst flux.InstanceID) ([]flux.Event, error) {
	return db.queryEvents(
		`SELECT id, service_ids, type, started_at, ended_at, log_level, message, metadata
		FROM events
		WHERE instance_id = $1
		ORDER BY started_at DESC`,
		string(inst),
	)
}

func (db *pgDB) GetEvent(id flux.EventID) (flux.Event, error) {
	es, err := db.queryEvents(
		`SELECT id, service_ids, type, started_at, ended_at, log_level, message, metadata
		FROM events
		WHERE id = $1`,
		string(id),
	)
	if err != nil {
		return flux.Event{}, err
	}
	if len(es) <= 0 {
		return flux.Event{}, fmt.Errorf("event not found")
	}
	return es[0], nil
}

func (db *pgDB) LogEvent(inst flux.InstanceID, e flux.Event) error {
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
		serviceIDs = append(serviceIDs, string(id))
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
