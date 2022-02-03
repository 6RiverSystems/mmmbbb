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
	"os"
	"path"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v4/stdlib"
	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"

	"go.6river.tech/gosix/db"
	"go.6river.tech/gosix/db/postgres"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/version"
)

func ClientForTest(t testing.TB, opts ...ent.Option) *ent.Client {
	var driverName, dialectName, dsn string
	if env := os.Getenv("NODE_ENV"); env == "acceptance" {
		driverName = "pgx"
		dialectName = dialect.Postgres
		dsn = db.PostgreSQLDSN("acceptance")
	} else {
		driverName = db.SQLiteDriverName
		dialectName = dialect.SQLite
		// cannot use memory DBs for this app due to
		// https://github.com/mattn/go-sqlite3/issues/923
		dsn = db.SQLiteDSN(path.Join(t.TempDir(), version.AppName+"_test"), true, false)
	}
	conn, err := db.Open(driverName, dialectName, dsn)
	if err != nil {
		t.Fatalf("Failed to open %s for ent: %v", driverName, err)
	}
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
	if dialectName != dialect.Postgres {
		if err := db.MigrateUpEnt(testutils.ContextForTest(t), client.Schema); err != nil {
			t.Fatalf("Failed to apply migrations: %v", err)
		}
	}
	ResetTables(t, client)
	return client
}

func ResetTables(t testing.TB, client *ent.Client) {
	// just in case, make sure the tables are all empty
	deletes := []func(context.Context) (int, error){
		client.Delivery.Delete().Exec,
		client.Message.Delete().Exec,
		client.Subscription.Delete().Exec,
		client.Topic.Delete().Exec,
	}
	for _, d := range deletes {
		if _, err := d(testutils.ContextForTest(t)); err != nil {
			t.Fatalf("Failed to cleanup old test data")
		}
	}
}
