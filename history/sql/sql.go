package sql

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/weaveworks/flux/history"
)

// A history DB that uses a SQL database
type DB struct {
	driver *sqlx.DB
	squirrel.StatementBuilderType
}

func NewSQL(driver, datasource string) (history.DB, error) {
	db, err := sqlx.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	historyDB := &DB{
		driver:               db,
		StatementBuilderType: statementBuilder(db),
	}
	switch driver {
	case "ql", "ql-mem":
		q := &qlDB{historyDB}
		return q, q.sanityCheck()
	default:
		p := &pgDB{historyDB}
		return p, p.sanityCheck()
	}
}

var statementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.driver.Query(query, args...)
}

func (db *DB) Close() error {
	return db.driver.Close()
}
