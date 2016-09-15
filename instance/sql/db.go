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

type db struct {
	conn   *sql.DB
	schema string
}

func New(driver, datasource string) (*db, error) {
	conn, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	db := &db{
		conn:   conn,
		schema: schemaByDriver[driver],
	}
	if db.schema == "" {
		return nil, ErrNoSchemaDefinedForDriver
	}
	return db, db.ensureTables()
}

func (db *db) Set(inst flux.InstanceID, conf instance.InstanceConfig) error {
	bytes, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM config WHERE instance = $1`, string(inst))
	if err == nil {
		_, err = tx.Exec(`INSERT INTO config (instance, config, stamp) VALUES
                       ($1, $2, now())`, string(inst), string(bytes))
	}
	if err == nil {
		err = tx.Commit()
	}
	return err
}

func (db *db) Get(inst flux.InstanceID) (instance.InstanceConfig, error) {
	var c string
	err := db.conn.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&c)
	if err != nil {
		return instance.InstanceConfig{}, err
	}
	var conf instance.InstanceConfig
	return conf, json.Unmarshal([]byte(c), &conf)
}

// ---

func (db *db) ensureTables() error {
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
