// Package migrations holds the embedded SQL migrations that back the
// MySQL database.
package migrations

import (
	"embed"
	"fmt"
	"log/slog"

	sqldriver "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

//go:embed *.sql
var migrationsFS embed.FS

// Run performs all migrations in the given filesystem against a MySQL database.
//
// dsn is the same DSN used to open dbx. It's used to tell golang-migrate
// which database to track migrations against: left unset, it falls back to
// asking the connection via `SELECT DATABASE()`, which can come back empty
// depending on the connection/pool (e.g. some proxies don't preserve the
// session's selected database), causing migrations to fail even though the
// DSN itself names a database.
func Run(dbx *sqlx.DB, dsn string) error {
	cfg, err := sqldriver.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("error parsing database dsn: %s", err)
	}

	d, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("error creating migrations source: %s", err)
	}
	i, err := mysql.WithInstance(dbx.DB, &mysql.Config{
		DatabaseName: cfg.DBName,
	})
	if err != nil {
		return fmt.Errorf("error creating mysql instance for migration: %s", err)
	}
	migrator, err := migrate.NewWithInstance("iofs", d, "mysql", i)
	if err != nil {
		return fmt.Errorf("error creating migrator: %s", err)
	}
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("error migrating: %s", err)
	}
	slog.Info("migrated")

	return nil
}
