package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/joshjon/kit/sqlitedb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joshjon/verve/internal/sqlite/migrations"
)

// NewTestDB creates an in-memory SQLite database with all migrations applied.
// The database is automatically closed when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	db, err := sqlitedb.Open(ctx, sqlitedb.WithInMemory())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, db.Close())
	})

	err = sqlitedb.Migrate(db, migrations.FS)
	require.NoError(t, err)
	return db
}
