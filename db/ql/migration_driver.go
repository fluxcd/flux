package ql

import (
	"database/sql"

	"github.com/mattes/migrate/driver"
	"github.com/mattes/migrate/file"
	"github.com/mattes/migrate/migrate/direction"
)

// These are registered under the URL schemes, since that's how they
// are looked up by the `migrate` module. What we *actually* want is
// the name that the database/sql driver was registered with, so
// that's supplied as the field `driver`.
func init() {
	driver.RegisterDriver("file", &Driver{driver: "ql"})
	driver.RegisterDriver("memory", &Driver{driver: "ql-mem"})
}

type Driver struct {
	driver string
	conn   *sql.DB
}

func (d *Driver) Initialize(source string) (err error) {
	d.conn, err = sql.Open(d.driver, source)
	if err != nil {
		return err
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS
                        schema_migration (stamp time NOT NULL, version int NOT NULL)`)
	if err == nil {
		_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS pk_schema_migration ON schema_migration (version)`)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Driver) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

func (d *Driver) FilenameExtension() string {
	return "sql"
}

func (d *Driver) Migrate(f file.File, pipe chan interface{}) {
	defer close(pipe)
	pipe <- f

	tx, err := d.conn.Begin()
	if err != nil {
		pipe <- err
		return
	}

	if f.Direction == direction.Up {
		if _, err := tx.Exec("INSERT INTO schema_migration (stamp, version) VALUES (now(), $1)", f.Version); err != nil {
			pipe <- err
			if err := tx.Rollback(); err != nil {
				pipe <- err
			}
			return
		}
	} else if f.Direction == direction.Down {
		if _, err := tx.Exec("DELETE FROM schema_migration WHERE version=$1", f.Version); err != nil {
			pipe <- err
			if err := tx.Rollback(); err != nil {
				pipe <- err
			}
			return
		}
	}

	if err := f.ReadContent(); err != nil {
		pipe <- err
		return
	}

	if _, err := tx.Exec(string(f.Content)); err != nil {
		pipe <- err
		if err := tx.Rollback(); err != nil {
			pipe <- err
		}
		return
	}

	if err := tx.Commit(); err != nil {
		pipe <- err
		return
	}
}

func (d *Driver) Version() (uint64, error) {
	var version uint64
	err := d.conn.QueryRow("SELECT version FROM schema_migration ORDER BY version DESC LIMIT 1").Scan(&version)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return version, nil
	}
}
