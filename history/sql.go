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

func (db *sqlDB) AllEvents(namespace string) (map[string]History, error) {
	hs := make(map[string]History)

	var err error
	var stateRows, eventRows *sql.Rows
	stateRows, err = db.driver.Query(`SELECT service, state FROM state
                                WHERE namespace = $1`, namespace)
	if err == nil {
		eventRows, err = db.driver.Query(`SELECT service, message, stamp
                                FROM history
                                WHERE namespace = $1
                                ORDER BY service, stamp DESC`, namespace)
	}

	if err != nil {
		return nil, err
	}

	for stateRows.Next() {
		var service string
		var state string
		_ = stateRows.Scan(&service, &state)
		hs[service] = History{Service: service, State: ServiceState(state)}
	}

	for eventRows.Next() {
		var service string
		var event Event
		_ = eventRows.Scan(&service, &event.Msg, &event.Stamp)
		if h, ok := hs[service]; ok {
			h.Events = append(h.Events, event)
			hs[service] = h
		} else {
			hs[service] = History{
				Service: service,
				State:   StateUnknown,
				Events:  []Event{event},
			}
		}
	}

	if err = eventRows.Err(); err != nil {
		return nil, err
	}
	if err = stateRows.Err(); err != nil {
		return nil, err
	}
	return hs, nil
}

func (db *sqlDB) EventsForService(namespace, service string) (History, error) {
	h := History{Service: service, State: StateUnknown}

	stateRow := db.driver.QueryRow(`SELECT state
	                         FROM state
	                         WHERE namespace = $1 AND service = $2`, namespace, service)
	var s string
	if err := stateRow.Scan(&s); err != nil {
		if err != sql.ErrNoRows {
			return h, err
		}
	} else {
		h.State = ServiceState(s)
	}

	eventRows, err := db.driver.Query(`SELECT message, stamp
                                FROM history
                                WHERE namespace = $1 AND service = $2
                                ORDER BY stamp DESC`, namespace, service)
	if err != nil {
		return h, err
	}
	h.Events = make([]Event, 0)
	for eventRows.Next() {
		e := Event{}
		eventRows.Scan(&e.Msg, &e.Stamp)
		h.Events = append(h.Events, e)
	}
	if eventRows.Err() != nil {
		return h, eventRows.Err()
	}
	return h, nil
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

func (db *sqlDB) ChangeState(namespace, service string, newState ServiceState) error {
	tx, err := db.driver.Begin()
	if err != nil {
		return err
	} else {
		row := tx.QueryRow(`SELECT state FROM state
                               WHERE namespace=$1 AND service=$2`, namespace, service)

		var _oldState string
		switch scanErr := row.Scan(&_oldState); scanErr {
		case sql.ErrNoRows:
			_, err = tx.Exec(`INSERT INTO state (namespace, service, state) VALUES ($1, $2, $3)`, namespace, service, string(newState))
		case nil:
			_, err = tx.Exec(`UPDATE state SET state=$3
                        WHERE namespace=$1 AND service=$2`, namespace, service, string(newState))
		default:
			err = scanErr
		}
	}

	if err == nil {
		err = tx.Commit()
	} else {
		tx.Rollback()
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
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS state
             (namespace string NOT NULL,
              service   string NOT NULL,
              state     string NOT NULL)`)
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
