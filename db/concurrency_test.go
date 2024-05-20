package db

import (
	"context"
	"database/sql"
	"errors"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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

	eg, ctx := errgroup.WithContext(ctx)

	// 1. Start a transaction to block things
	// 2. Attempt to start a second transaction
	// 3. Concurrently commit the first tx and cancel the context for the second

	tx1, err := db.BeginTx(ctx, nil)
	require.NoError(t, err, "begin tx1")

	ctx2, cancel2 := context.WithCancel(ctx)
	eg.Go(func() error {
		tx2, err := db.BeginTx(ctx2, nil)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
		}
		err = tx2.Rollback()
		return err
	})

	time.Sleep(100 * time.Millisecond)

	require.NoError(t, tx1.Commit())
	cancel2()
	require.NoError(t, eg.Wait())
}
