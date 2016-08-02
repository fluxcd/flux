package history

import (
	"database/sql"

	"github.com/go-kit/kit/log"
)

// A history DB that uses a SQL database

type sqlDB struct {
	driver *sql.DB
	logger log.Logger
}

func NewSQL(driver, datasource string, logger log.Logger) (DB, error) {
	db, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	if err := ensureTables(db, logger); err != nil {
		return nil, err
	}
	return &sqlDB{driver: db, logger: logger}, nil
}

func (db *sqlDB) AllEvents(namespace string) ([]Event, error) {
	eventRows, err := db.driver.Query(`SELECT service, message, stamp
                                FROM history
                                WHERE namespace = $1
                                ORDER BY service, stamp DESC`, namespace)

	if err != nil {
		return nil, err
	}

	events := make([]Event, 0)
	for eventRows.Next() {
		var event Event
		eventRows.Scan(&event.Service, &event.Msg, &event.Stamp)
		events = append(events, event)
	}

	if err = eventRows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (db *sqlDB) EventsForService(namespace, service string) ([]Event, error) {
	eventRows, err := db.driver.Query(`SELECT message, stamp
                                FROM history
                                WHERE namespace = $1 AND service = $2
                                ORDER BY stamp DESC`, namespace, service)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0)
	for eventRows.Next() {
		event := Event{}
		eventRows.Scan(&event.Msg, &event.Stamp)
		events = append(events, event)
	}
	if eventRows.Err() != nil {
		return nil, eventRows.Err()
	}
	return events, nil
}

func (db *sqlDB) LogEvent(namespace, service, msg string) error {
	tx, err := db.driver.Begin()
	if err != nil {
		return err
	} else {
		_, err = tx.Exec(`INSERT INTO history
                       (namespace, service, message, stamp)
                       VALUES ($1, $2, $3, now())`, namespace, service, msg)
	}
	if err == nil {
		err = tx.Commit()
	}
	return err
}

func ensureTables(db *sql.DB, logger log.Logger) (err error) {
	logger = log.NewContext(logger).With("method", "ensureTables")
	defer logger.Log("err", err)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS history
             (namespace string NOT NULL,
              service   string NOT NULL,
              message   string NOT NULL,
              stamp     time NOT NULL)`)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
