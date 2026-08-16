// Package mysql is the MySQL implementation of the seymour service
// interfaces, replacing internal/sqlite.
package mysql

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/jdholdren/seymour/internal/seymour"
)

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
