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
	"fmt"
	"io/fs"

	"github.com/jmoiron/sqlx"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/migrate"
)

func MigrateUp(
	ctx context.Context,
	// dirty hack, ORM reference, currently must be a *ent.Client
	migrateVia interface{},
	migrationsFS fs.FS,
) error {
	migrations, err := migrate.LoadFS(migrationsFS, nil)
	if err != nil {
		return err
	}

	m := (&migrate.Migrator{}).AddAndSort("", migrations...)

	return Up(ctx, migrateVia, m)
}

func Up(
	ctx context.Context,
	// dirty hack, ORM reference, currently must be a *ent.Client
	migrateVia interface{},
	migrator *migrate.Migrator,
) error {
	sql, driverName, dialectName, err := OpenDefault()
	if sql != nil {
		// TODO: error check
		defer sql.Close()
	}
	if err != nil {
		return fmt.Errorf("failed to connect to DB for Up migration: %w", err)
	}

	if !migrator.HasMigrations() || dialectName == SqliteDialect {
		switch mdb := migrateVia.(type) {
		case *ent.Client:
			err := MigrateUpEnt(ctx, mdb.Schema)
			if err != nil {
				return fmt.Errorf("failed Up migration via ent for %s: %w", dialectName, err)
			}
			return nil
		default:
			return fmt.Errorf("unrecognized migrateVia for %s: %T", dialectName, migrateVia)
		}
	}

	switch dialectName {
	case PostgresDialect:
		migrator = migrator.WithDialect(&migrate.PgxDialect{})
	default:
		// TODO: if we had a dialect registry could make this generic
		return fmt.Errorf("unrecognized dialect '%s'", dialectName)
	}

	// default fallthrough assumes we've initialized migrator with a dialect
	return migrator.Up(ctx, sqlx.NewDb(sql, driverName))
}
