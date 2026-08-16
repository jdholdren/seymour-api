package mysql_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	migrationsmysql "github.com/jdholdren/seymour/internal/migrations-mysql"
	"github.com/jdholdren/seymour/internal/mysql"
)

// testRepo spins up an ephemeral MySQL container via testcontainers-go,
// applies the mysql migration set, and returns a Repo wired to it.
//
// parseTime=true is required on the DSN: seymour.DBTime.Scan expects either
// a time.Time or an RFC3339 string, and without parseTime=true the MySQL
// driver hands back a []byte, which Scan doesn't currently handle. Flagging
// this now as a known follow-up for the real repo implementation phase.
func testRepo(t *testing.T) mysql.Repo {
	t.Helper()
	ctx := context.Background()

	container, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("seymour_test"),
		tcmysql.WithUsername("seymour"),
		tcmysql.WithPassword("seymour"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	connStr, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	require.NoError(t, err)

	db, err := sqlx.Open("mysql", connStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.PingContext(ctx), fmt.Sprintf("failed to ping mysql at %s", connStr))
	require.NoError(t, migrationsmysql.Run(db))

	return mysql.New(db)
}
