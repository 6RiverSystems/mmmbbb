package db

import (
	"database/sql"
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
	db.SetConnMaxIdleTime(1)
	require.NoError(t, err, "open sqlite")
	t.Cleanup(func() { db.Close() })

	// capture test timeout, etc
	ctx := testutil.Context(t)

	// create a table we can poke at
	_, err = db.ExecContext(ctx, "create table data (key text primary key, value text not null)")
	require.NoError(t, err, "create table")

	eg, ctx := errgroup.WithContext(ctx)

	// first goroutine starts a transaction, inserts some data, and sleeps so the
	// others will block. Other goroutines try to start transactions to read this
	// data.

	begun := make(chan struct{})
	eg.Go(func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		close(begun)
		_, err = tx.ExecContext(ctx, "insert into data (key, value) values ('key', 'value')")
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		time.Sleep(50 * time.Millisecond)
		return tx.Commit()
	})
	// let it get going
	select {
	case <-begun:
		// ok
	case <-ctx.Done():
		require.NoError(t, eg.Wait(), "table init")
		t.Fatal("should have failed before this")
	}

	// start a handful of goroutines competing to access the data
	for i := 0; i < 5; i++ {
		eg.Go(func() error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			rows, err := tx.QueryContext(ctx, "select value from data where key = 'key'")
			require.NoError(t, err, "query")
			defer rows.Close()
			var value string
			require.True(t, rows.Next(), "next")
			require.NoError(t, rows.Scan(&value), "scan")
			require.Equal(t, "value", value)
			require.False(t, rows.Next(), "no more")
			return tx.Commit()
		})
	}
	require.NoError(t, eg.Wait(), "queries")
}
