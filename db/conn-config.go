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

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

	"go.6river.tech/mmmbbb/db/postgres"
	"go.6river.tech/mmmbbb/logging"
)

var defaultDbName string

var (
	// these names need to match what ent uses

	PostgresDialect = "postgres"
	SqliteDialect   = "sqlite3"
)

func DefaultDbUrl() string {
	url := os.Getenv("DATABASE_URL")
	if url != "" {
		return ApplyPgAppName(url)
	}

	// TODO: don't use NODE_ENV for things that aren't nodejs
	env := os.Getenv("NODE_ENV")
	if env == "" {
		// assume dev mode if unspecified
		env = "development"
	}

	if defaultDbName == "" {
		panic("Must call SetDefaultDbName before configuring DB")
	}

	if env == "test" {
		// use sqlite instead. don't use memory mode because of
		// https://github.com/mattn/go-sqlite3/issues/923
		return SQLiteDSN(defaultDbName, false, false)
	}

	// generate something like postgres://localhost/foo_development
	return ApplyPgAppName(PostgreSQLDSN(env))
}

func PostgreSQLDSN(suffix string) string {
	return fmt.Sprintf(
		"postgres://6river:6river@localhost/%s_%s?sslmode=disable",
		defaultDbName,
		suffix,
	)
}

func ApplyPgAppName(dbUrl string) string {
	parsed, err := url.Parse(dbUrl)
	if err != nil {
		// TODO: don't panic
		panic(err)
	}
	q, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		// TODO: don't panic
		panic(err)
	}
	if q.Has("application_name") {
		// don't change it
		return dbUrl
	}
	podName := os.Getenv("POD_NAME")
	conName := os.Getenv("CONTAINER_NAME")
	appName := ""
	if podName != "" && conName != "" {
		appName = podName + "/" + conName
	} else if podName != "" {
		appName = podName
	} else if conName != "" {
		appName = conName
	}
	if appName == "" {
		return dbUrl
	}
	q.Set("application_name", appName)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// SQLiteDSN is implemented separately for CGO and non-CGO

func SetDefaultDbName(name string) {
	if defaultDbName != "" {
		panic("Already called SetDefaultDbName")
	}
	defaultDbName = name
}

func GetDefaultDbName() string {
	if defaultDbName == "" {
		panic("No default db name set")
	}
	return defaultDbName
}

func ParseDefault() (driverName, dialectName, dsn string, err error) {
	dsn = DefaultDbUrl()

	if strings.HasPrefix(dsn, "sqlite:") {
		dsn = "file:" + strings.TrimPrefix(dsn, "sqlite:")
		driverName = SQLiteDriverName
		// driverName = "sqlite" // for modernc driver, in the future
		dialectName = SqliteDialect
	} else if strings.HasPrefix(dsn, "postgres:") {
		driverName = "pgx"
		dialectName = PostgresDialect
	} else {
		return "", "", dsn, fmt.Errorf("unrecognized db url '%s'", dsn)
	}
	return
}

func OpenDefault() (db *sql.DB, driverName, dialectName string, err error) {
	var dsn string
	if driverName, dialectName, dsn, err = ParseDefault(); err != nil {
		return
	} else {
		db, err = Open(driverName, dialectName, dsn)
		return
	}
}

func Open(driverName, dialectName, dsn string) (db *sql.DB, err error) {
	if driverName == "pgx" {
		// wire up the OnNotice logger
		var cfg *pgx.ConnConfig
		cfg, err = pgx.ParseConfig(dsn)
		if err != nil {
			return
		}
		logger := logging.GetLogger("pgx/notice")
		cfg.OnNotice = func(pc *pgconn.PgConn, n *pgconn.Notice) {
			logger.Info().Interface("notice", n).Msg(n.Message)
		}
		db = stdlib.OpenDB(*cfg)
	} else {
		db, err = sql.Open(driverName, dsn)
		if err != nil {
			err = fmt.Errorf("failed to open default DB connection: %w", err)
		}
	}

	maxOpenConns := 10
	maxIdleTime := 10 * time.Second
	if driverName == "sqlite3" {
		if strings.Contains(dsn, "mode=memory") {
			// memory mode can only have one connection at a time, unless shared cache
			// mode is in use, because otherwise extra connections will see separate
			// DBs. Shared cache can alleviate that, but creates "table is locked"
			// errors that are largely unfixable.
			maxOpenConns = 1
			// we don't want to kill the last connection ever, we'd lose the data
			maxIdleTime = 0
		}
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	db.SetConnMaxIdleTime(maxIdleTime)
	// don't think we need to use db.SetConnMaxLifetime()

	return
}

// WaitForDB repeatedly tries to connect to the default DB until it either
// succeeds, or the given context is canceled.
func WaitForDB(ctx context.Context) error {
	logger := logging.GetLogger("dbwait")
RETRY:
	for {
		db, _, _, err := OpenDefault()
		if err == nil {
			err = db.PingContext(ctx)
		}
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		retriable := false
		var errno syscall.Errno
		var netErr net.Error
		if errors.As(err, &errno) && errno == syscall.ECONNREFUSED {
			retriable = true
		} else if errors.As(err, &netErr) && netErr.Timeout() {
			retriable = true
		} else if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.CannotConnectNow); ok {
			// database startup in progress
			retriable = true
		} else if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.InvalidCatalogName); ok {
			if createVia := os.Getenv("CREATE_DB_VIA"); createVia != "" {
				// database doesn't exist and we have a rule for how to create it
				driverName, dialect, dsn, parseErr := ParseDefault()
				if parseErr == nil {
					// try to connect to CREATE_DB_VIA and create our db
					if createErr := TryCreateDB(ctx, driverName, dialect, dsn, createVia); createErr == nil {
						logger.Info().
							Err(err).
							Str("createVia", createVia).
							Msg("Auto-created database after connect error, retrying db connect")
						continue RETRY
					}
				}
			}
		}

		if !retriable {
			return err
		}

		logger.Warn().Err(err).Msg("DB not ready yet, will retry")

		// retry every second
		retry := time.After(time.Second)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-retry:
			continue RETRY
		}
	}
}

func TryCreateDB(ctx context.Context, driverName, dialect, dsn, createVia string) error {
	if driverName == "pgx" && dialect == PostgresDialect {
		cfg, err := pgx.ParseConfig(dsn)
		if err != nil {
			return err
		}
		var origDB string
		origDB, cfg.Database = cfg.Database, createVia
		db := stdlib.OpenDB(*cfg)
		defer db.Close()
		// can't use placeholders for CREATE DATABASE, have to escape (quote) things instead
		_, err = db.ExecContext(
			ctx,
			fmt.Sprintf("CREATE DATABASE %s", postgres.QuoteIdentifier(origDB)),
		)
		// TODO: detect "already exists" and report that as success (e.g. multiple instances racing to create)
		return err
	} else {
		return errors.New("don't know how to create this kind of db")
	}
}
