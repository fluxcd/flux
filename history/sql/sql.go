package sql

import (
	"database/sql"

	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy/history"
)

// A history DB that uses a SQL database
type DB struct {
	driver *sql.DB
}

func NewSQL(driver, datasource string) (*DB, error) {
	db, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	historyDB := &DB{
		driver: db,
	}
	return historyDB, historyDB.sanityCheck()
}

func (db *DB) queryEvents(query string, params ...interface{}) ([]history.Event, error) {
	eventRows, err := db.driver.Query(query, params...)

	if err != nil {
		return nil, err
	}
	defer eventRows.Close()

	events := []history.Event{}
	for eventRows.Next() {
		var event history.Event
		eventRows.Scan(&event.Service, &event.Msg, &event.Stamp)
		events = append(events, event)
	}

	if err = eventRows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (db *DB) AllEvents() ([]history.Event, error) {
	return db.queryEvents(`SELECT service, message, stamp
                           FROM history
                           ORDER BY stamp DESC`)
}

func (db *DB) EventsForService(namespace, service string) ([]history.Event, error) {
	return db.queryEvents(`SELECT service, message, stamp
                           FROM history
                           WHERE namespace = $1 AND service = $2
                           ORDER BY stamp DESC`, namespace, service)
}

func (db *DB) LogEvent(namespace, service, msg string) error {
	tx, err := db.driver.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`INSERT INTO history
                       (namespace, service, message, stamp)
                       VALUES ($1, $2, $3, now())`, namespace, service, msg)
	if err == nil {
		err = tx.Commit()
	}
	return err
}

func (db *DB) sanityCheck() (err error) {
	_, err = db.driver.Query("SELECT namespace, service, message, stamp FROM history LIMIT 1")
	if err != nil {
		return errors.Wrap(err, "sanity checking history table")
	}
	return nil
}

func (db *DB) Close() error {
	return db.driver.Close()
}
