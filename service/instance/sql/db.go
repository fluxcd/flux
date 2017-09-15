package sql

import (
	"database/sql"
	"encoding/json"

	_ "github.com/cznic/ql/driver"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/instance"
)

type DB struct {
	conn *sql.DB
}

func New(driver, datasource string) (*DB, error) {
	conn, err := sql.Open(driver, datasource)
	if err != nil {
		return nil, err
	}
	db := &DB{
		conn: conn,
	}
	return db, db.sanityCheck()
}

func (db *DB) UpdateConfig(inst service.InstanceID, update instance.UpdateFunc) error {
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
		currentConfig = instance.Config{}
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

func (db *DB) GetConfig(inst service.InstanceID) (instance.Config, error) {
	var c string
	err := db.conn.QueryRow(`SELECT config FROM config WHERE instance = $1`, string(inst)).Scan(&c)
	switch err {
	case nil:
		break
	case sql.ErrNoRows:
		return instance.Config{}, nil
	default:
		return instance.Config{}, err
	}
	var conf instance.Config
	return conf, json.Unmarshal([]byte(c), &conf)
}

func (db *DB) UpdateGitUrl(inst service.InstanceID, url string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM giturl WHERE instance = $1`, string(inst))
	if err == nil {
		_, err = tx.Exec(`INSERT INTO giturl (instance, giturl, stamp) VALUES
                            ($1, $2, now())`, string(inst), url)
	}
	if err == nil {
		err = tx.Commit()
	} else {
		err = tx.Rollback()
	}
	return err
}

func (db *DB) GetGitUrl(inst service.InstanceID) (string, error) {
	var u string
	err := db.conn.QueryRow(`SELECT giturl FROM config WHERE instance = $1`, string(inst)).Scan(&u)
	switch err {
	case nil:
		return u, nil
	case sql.ErrNoRows:
		return "", nil
	default:
		return "", err
	}
}

// ---

func (db *DB) sanityCheck() error {
	_, err := db.conn.Query(`SELECT instance, config, stamp FROM config LIMIT 1`)
	if err != nil {
		return errors.Wrap(err, "failed sanity check for config table")
	}
	return nil
}
