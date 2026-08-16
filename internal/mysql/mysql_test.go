package mysql_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/jdholdren/seymour/internal/migrations"
	"github.com/jdholdren/seymour/internal/mysql"
)

// testDB is shared across every test in this package: TestMain starts one
// MySQL container and applies migrations once, rather than paying for a
// fresh container per test.
var testDB *sqlx.DB

func TestMain(m *testing.M) {
	// os.Exit below skips deferred calls, so cleanup is run explicitly
	// rather than via defer.
	code, err := runTests(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

// runTests starts the shared MySQL container, runs the suite, and tears the
// container down before returning — kept separate from TestMain so cleanup
// always runs, since TestMain's os.Exit would otherwise skip any defers.
func runTests(m *testing.M) (int, error) {
	ctx := context.Background()

	container, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("seymour_test"),
		tcmysql.WithUsername("seymour"),
		tcmysql.WithPassword("seymour"),
	)
	if err != nil {
		return 0, fmt.Errorf("error starting mysql container: %w", err)
	}
	defer func() {
		if tErr := container.Terminate(context.Background()); tErr != nil {
			log.Printf("error terminating mysql container: %s", tErr)
		}
	}()

	// parseTime=true is required so the MySQL driver scans DATETIME/TIMESTAMP
	// columns directly into time.Time; without it the driver hands back a
	// []byte instead.
	connStr, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		return 0, fmt.Errorf("error building mysql connection string: %w", err)
	}

	db, err := sqlx.Open("mysql", connStr)
	if err != nil {
		return 0, fmt.Errorf("error opening mysql connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("failed to ping mysql at %s: %w", connStr, err)
	}
	if err := migrations.Run(db); err != nil {
		return 0, fmt.Errorf("error running mysql migrations: %w", err)
	}

	testDB = db

	return m.Run(), nil
}

// tables lists every table truncated between tests, in an order that would
// satisfy FK dependency order if the schema ever gains declared foreign keys
// (it doesn't today; ordering here is just future-proofing).
var tables = []string{
	"filter_keywords",
	"user_filters",
	"timeline_entries",
	"subscriptions",
	"feed_entries",
	"feeds",
	"user_logins",
	"users",
}

// testRepo returns a Repo wired to the shared testDB, registering a Cleanup
// that truncates every table so each test starts from an empty schema
// without spinning up its own container.
func testRepo(t *testing.T) mysql.Repo {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()
		for _, table := range tables {
			_, err := testDB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s;", table))
			require.NoError(t, err)
		}
	})

	return mysql.New(testDB)
}
