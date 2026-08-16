// Package mysql is the in-progress MySQL implementation of the seymour
// service interfaces, replacing internal/sqlite. It currently starts as a
// null implementation: every method returns ErrNotImplemented so that the
// tests in this package fail red until the real queries are written.
package mysql

import (
	"errors"

	"github.com/jmoiron/sqlx"
	_ "github.com/go-sql-driver/mysql"

	"github.com/jdholdren/seymour/internal/seymour"
)

// ErrNotImplemented is returned by every Repo method until the real MySQL
// queries are written.
var ErrNotImplemented = errors.New("mysql: not implemented")

// Repo implements seymour.UserService, seymour.FeedService, and
// seymour.TimelineService against a MySQL database.
type Repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) Repo {
	return Repo{db: db}
}

var (
	_ seymour.UserService     = Repo{}
	_ seymour.FeedService     = Repo{}
	_ seymour.TimelineService = Repo{}
)
