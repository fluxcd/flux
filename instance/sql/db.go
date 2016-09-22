package sql

import (
	"database/sql"
	"encoding/json"
	"os"

	_ "github.com/cznic/ql/driver"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/weaveworks/fluxy"
	"github.com/weaveworks/fluxy/instance"
)

var (
	ErrNoSchemaDefinedForDriver = errors.New("schema not defined for driver")

	qlSchema = `
      CREATE TABLE IF NOT EXISTS config
        (instance string NOT NULL,
         config   string NOT NULL,
         stamp    time NOT NULL)
    `

	pgSchema = `
      CREATE TABLE IF NOT EXISTS config
        (instance varchar(255) NOT NULL,
         config   text NOT NULL,
         stamp    timestamp with time zone NOT NULL)
    `

	schemaByDriver = map[string]string{
		"ql":       qlSchema,
		"ql-mem":   qlSchema,
		"postgres": pgSchema,
	}
)

type DB struct {
	conn   *sql.DB
	schema string
}

func New(driver, datasource string) (*DB, error) {
	conn, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{
		conn:   conn,
		schema: schemaByDriver[driver],
	}
	if db.schema == "" {
		return nil, ErrNoSchemaDefinedForDriver
	}
	return db, db.ensureTables()
}

func (db *DB) Update(inst flux.InstanceID, update instance.UpdateFunc) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	var (
		currentConfig instance.Config
		confString    string
	)
	switch tx.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&confString) {
	case sql.ErrNoRows:
		currentConfig = instance.MakeConfig()
	case nil:
		if err = json.Unmarshal([]byte(confString), &currentConfig); err != nil {
			return err
		}
	default:
		return err
	}

	newConfig, err := update(currentConfig)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return errors.Wrapf(err, "transaction rollback failed: %s", err2)
		}
		return err
	}

	newConfigBytes, err := json.Marshal(newConfig)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM config WHERE instance = $1`, string(inst))
	if err == nil {
		_, err = tx.Exec(`INSERT INTO config (instance, config, stamp) VALUES
                       ($1, $2, now())`, string(inst), string(newConfigBytes))
	}
	if err == nil {
		err = tx.Commit()
	}
	return err
}

func (db *DB) Get(inst flux.InstanceID) (instance.Config, error) {
	var c string
	err := db.conn.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&c)
	if err != nil {
		return instance.Config{}, err
	}
	var conf instance.Config
	return conf, json.Unmarshal([]byte(c), &conf)
}

// ---

func (db *DB) ensureTables() error {
	// ql driver needs this to work correctly in a container
	os.MkdirAll(os.TempDir(), 0777)
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(db.schema)
	if err != nil {
		return err
	}
	return tx.Commit()
}
