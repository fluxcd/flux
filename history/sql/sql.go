package sql

import (
	"github.com/jmoiron/sqlx"

	"github.com/weaveworks/flux/history"
)

// A history DB that uses a SQL database
type DB struct {
	driver *sqlx.DB
}

func NewSQL(driver, datasource string) (history.DB, error) {
	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	historyDB := &DB{driver: db}
	switch driver {
	case "ql", "ql-mem":
		q := &qlDB{historyDB}
		return q, q.sanityCheck()
	default:
		p := &pgDB{historyDB}
		return p, p.sanityCheck()
	}
}

func (db *DB) Close() error {
	return db.driver.Close()
}
