// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package enttest

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	dbSql "database/sql"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"

	"go.6river.tech/mmmbbb/db"
	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/internal/testutil"
	"go.6river.tech/mmmbbb/migrations"
	"go.6river.tech/mmmbbb/version"
)

func StartDockertest(t testing.TB) string {
	// if no DB url provided, use dockertest to spin one up
	if testing.Short() {
		t.Skip("dockertest setup is not short, skipping test")
	}
	t.Log("using dockertest to create postgresql 14 db")
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)
	require.NoError(t, pool.Client.PingWithContext(testutil.Context(t)))
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "14-alpine",
		Env: []string{
			"POSTGRES_USER=mmmbbb",
			"POSTGRES_PASSWORD=mmmbbb",
			"POSTGRES_DB=test",
		},
	}, func(hc *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err)
	t.Cleanup(func() { resource.Close() })
	hostAndPort := resource.GetHostPort("5432/tcp")
	dbUrl := fmt.Sprintf("postgres://mmmbbb:mmmbbb@%s/test?sslmode=disable", hostAndPort)
	// wait for the db server to be ready
	pool.MaxWait = 30 * time.Second
	require.NoError(t, pool.Retry(func() error {
		if db, err := dbSql.Open("pgx", dbUrl); err != nil {
			t.Log("DB not ready yet")
			return err
		} else {
			defer db.Close()
			return db.Ping()
		}
	}))
	t.Logf("DB is ready at %s", dbUrl)
	return dbUrl
}

func ClientForTest(t testing.TB, opts ...ent.Option) *ent.Client {
	var driverName, dialectName, dsn string
	if env := os.Getenv("NODE_ENV"); env == "acceptance" {
		driverName = "pgx"
		dialectName = dialect.Postgres
		if dsn = os.Getenv("DATABASE_URL"); dsn == "" {
			dsn = StartDockertest(t)
			t.Setenv("DATABASE_URL", dsn)
		}
	} else {
		driverName = db.SQLiteDriverName
		dialectName = dialect.SQLite
		// cannot use memory DBs for this app due to
		// https://github.com/mattn/go-sqlite3/issues/923
		dsn = db.SQLiteDSN(path.Join(t.TempDir(), version.AppName+"_test"), true, false)
		t.Logf("Using sqlite3 DB at %s", dsn)
	}
	conn, err := db.Open(driverName, dialectName, dsn)
	if err != nil {
		t.Fatalf("Failed to open %s for ent: %v", driverName, err)
	}
	conn.SetConnMaxIdleTime(time.Minute)
	conn.SetMaxOpenConns(2)
	// provide a default logging function, so tests can just use `ent.Debug()` if
	// they want simple logging
	opts = append([]ent.Option{
		ent.Driver(sql.OpenDB(dialectName, conn)),
		ent.Log(t.Log),
	}, opts...)
	if strings.Contains(os.Getenv("DEBUG"), "sql") {
		opts = append(opts, ent.Debug())
	}
	client := ent.NewClient(opts...)
	t.Cleanup(func() { client.Close() })
	if err = client.DB().Ping(); err != nil {
		if driverName == "pgx" {
			if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.InvalidCatalogName); ok {
				// we can't call skip from here, we need to check this again higher up
				t.Skip("Acceptance test DB does not exist, skipping test")
			}
		}
		t.Fatalf("Failed to connect %s for ent: %v", dialectName, err)
	}
	switch dialectName {
	case dialect.Postgres:
		if err := db.MigrateUp(testutil.Context(t), client, migrations.MessageBusMigrations); err != nil {
			t.Fatalf("Failed to apply migrations: %v", err)
		}
	default:
		if err := db.MigrateUpEnt(testutil.Context(t), client.Schema); err != nil {
			t.Fatalf("Failed to apply migrations: %v", err)
		}
	}
	ResetTables(t, client)
	return client
}

func ResetTables(t testing.TB, client *ent.Client) {
	// just in case, make sure the tables are all empty
	// Do it in a transaction so that we get the busy timeout handling that seems
	// to be missing from non-tx stuff sometimes?
	ctx := testutil.Context(t)
	err := client.DoCtxTx(ctx, nil, func(ctx context.Context, tx *ent.Tx) error {
		deletes := []struct {
			typ any
			f   func(context.Context) (int, error)
		}{
			{tx.Delivery, tx.Delivery.Delete().Exec},
			{tx.Message, tx.Message.Delete().Exec},
			{tx.Subscription, tx.Subscription.Delete().Exec},
			{tx.Snapshot, tx.Snapshot.Delete().Exec},
			{tx.Topic, tx.Topic.Delete().Exec},
		}
		for _, d := range deletes {
			if _, err := d.f(ctx); err != nil {
				t.Fatalf("Failed to cleanup old %[1]T test data: (%[2]T) %[2]v\nat %s", d.typ, err, string(debug.Stack()))
			}
		}
		return nil
	})
	require.NoError(t, err, "cleanup tables")
}
