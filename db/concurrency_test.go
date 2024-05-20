package db

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"go.6river.tech/mmmbbb/internal/testutil"
)

func TestSQLiteConcurrency(t *testing.T) {
	dsn := SQLiteDSN(path.Join(t.TempDir(), t.Name()), true, false)
	t.Logf("DSN=%q", dsn)
	db, err := sql.Open(SQLiteDriverName, dsn)
	db.SetMaxOpenConns(2)
	db.SetConnMaxIdleTime(time.Minute)
	require.NoError(t, err, "open sqlite")
	t.Cleanup(func() { db.Close() })

	// capture test timeout, etc
	ctx := testutil.Context(t)

	// create a table we can poke at
	_, err = db.ExecContext(ctx, "create table data (key text primary key, value text not null)")
	require.NoError(t, err, "create table")

	// load a bunch of data
	for i := range 1000 {
		_, err = db.ExecContext(ctx, "insert into data (key, value) values (?, ?)", strconv.Itoa(i), strconv.Itoa(rand.Intn(1000)))
		require.NoError(t, err, "insert data")
	}

	for range 1000 {

		// try to cancel a query and then roll back the transaction
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err, "begin")

		// loop until we manage to do a cancel
		for {
			ccx, cancel := context.WithCancel(ctx)
			go func() {
				time.Sleep(100 * time.Nanosecond)
				cancel()
			}()
			_, err := tx.QueryContext(ccx, "select * from data where key = value")
			if errors.Is(err, context.Canceled) {
				break
			}
		}

		err = tx.Rollback()
		require.NoError(t, err, "rollback")
	}
}
