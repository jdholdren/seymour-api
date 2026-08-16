// Package migrations holds the embedded SQL migrations that back the
// MySQL database.
package migrations

import (
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
)

//go:embed *.sql
var migrationsFS embed.FS

// Run performs all migrations in the given filesystem against a MySQL database.
func Run(dbx *sqlx.DB) error {
	d, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("error creating migrations source: %s", err)
	}
	i, err := mysql.WithInstance(dbx.DB, &mysql.Config{})
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
