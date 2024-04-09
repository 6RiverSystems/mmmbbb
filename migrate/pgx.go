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

package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.6river.tech/mmmbbb/db/postgres"
)

type PgxDialect struct{}

var _ Dialect = &PgxDialect{}

func (p *PgxDialect) DefaultConfig() *Config {
	return &Config{
		SchemaName: "public",
		TableName:  "migrations",
	}
}

// func (p *PgxDialect) Open(ctx context.Context, url string) (*sqlx.DB, error) {
// 	return sqlx.Open("pgx", url)
// }

func (p *PgxDialect) Verify(db *sqlx.DB) error {
	if _, ok := db.Driver().(*stdlib.Driver); !ok {
		return fmt.Errorf("Cannot use Pgx dialect without pgx driver")
	}
	return nil
}

func (p *PgxDialect) EnsureMigrationsTable(ctx context.Context, db *sqlx.DB, cfg *Config) error {
	// TODO: this won't deal if the table is wrong

	sql := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s.%s (
			id bigserial primary key,
			name text not null unique,
			run_on timestamptz not null
		)`,
		postgres.QuoteIdentifier(cfg.SchemaName),
		postgres.QuoteIdentifier(cfg.TableName),
	)

	_, err := db.ExecContext(ctx, sql)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" && pgErr.ConstraintName == "pg_type_typname_nsp_index" {
			// raced creating the migration tables, happens during `go test` sometimes, ignore it
			err = nil
		}
	}

	return err
}

func (p *PgxDialect) SelectStates(cfg *Config) string {
	return fmt.Sprintf(
		`SELECT id, name, run_on from %s.%s`,
		postgres.QuoteIdentifier(cfg.SchemaName),
		postgres.QuoteIdentifier(cfg.TableName),
	)
}

func (p *PgxDialect) InsertMigration(cfg *Config) string {
	return fmt.Sprintf(
		`INSERT INTO %s.%s (name, run_on) VALUES ($1, $2)`,
		postgres.QuoteIdentifier(cfg.SchemaName),
		postgres.QuoteIdentifier(cfg.TableName),
	)
}

func (p *PgxDialect) DeleteMigration(cfg *Config) string {
	return fmt.Sprintf(
		`DELETE FROM %s.%s WHERE name = $1`,
		postgres.QuoteIdentifier(cfg.SchemaName),
		postgres.QuoteIdentifier(cfg.TableName),
	)
}

func (p *PgxDialect) BeforeMigration(
	ctx context.Context,
	db *sqlx.DB,
	tx *sqlx.Tx,
	name string,
	direction Direction,
) error {
	_, err := tx.ExecContext(
		ctx,
		// can't use placeholders inside a string (which the pl/pgsql code here is),
		// so have to do quoting instead
		fmt.Sprintf(
			`DO $$ BEGIN RAISE NOTICE 'About to %% migration %%', %s, %s; END; $$`,
			postgres.QuoteString(direction.String()),
			postgres.QuoteString(name),
		),
	)
	return err
}
