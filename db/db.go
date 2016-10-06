// Code for initialising databases; individual components should put
// scripts in `db/migrations/{driver}`.  `db.MustMigrate` can then be
// used to make sure the database is up to date before components can
// use it.

package db

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattes/migrate/migrate"
	"github.com/pkg/errors"

	// This section imports the data/sql drivers and the migration
	// drivers.
	_ "github.com/cznic/ql/driver"
	_ "github.com/lib/pq"
	_ "github.com/mattes/migrate/driver/postgres"
	_ "github.com/weaveworks/fluxy/db/ql"
)

// Most SQL drivers expect the driver name to appear as the scheme in
// the database source URL; for instance,
// `postgres://host:2345`. However, cznic/ql uses the schemes "file"
// and "memory" (or just a bare path), and names its drivers `ql` and
// `ql-mem`. So we can deal just with URLs, translate these where
// needed.
func DriverForScheme(scheme string) string {
	switch scheme {
	case "file":
		return "ql"
	case "memory":
		return "ql-mem"
	default:
		return scheme
	}
}

// Make sure the database at the URL is up to date with respect to
// migrations, or return an error. The migration scripts are taken
// from `basedir/{scheme}`, with the scheme coming from the URL.
func Migrate(dburl, basedir string) (uint64, error) {
	u, err := url.Parse(dburl)
	if err != nil {
		return 0, errors.Wrap(err, "parsing database URL")
	}
	migrationsPath := filepath.Join(basedir, DriverForScheme(u.Scheme))
	if _, err := os.Stat(migrationsPath); err != nil {
		if os.IsNotExist(err) {
			return 0, errors.Wrapf(err, "migrations dir %s does not exist; driver %s not supported", migrationsPath, u.Scheme)
		}
		return 0, errors.Wrapf(err, "verifying migrations directory %s exists", migrationsPath)
	}

	errs, _ := migrate.UpSync(dburl, migrationsPath)
	if len(errs) > 0 {
		return 0, errors.Wrap(compositeError{errs}, "migrating database")
	}
	version, err := migrate.Version(dburl, migrationsPath)
	if err != nil {
		return 0, err
	}
	return version, nil
}

type compositeError struct {
	errors []error
}

func (errs compositeError) Error() string {
	msgs := make([]string, len(errs.errors))
	for i := range errs.errors {
		msgs[i] = errs.errors[i].Error()
	}
	return strings.Join(msgs, "; ")
}
