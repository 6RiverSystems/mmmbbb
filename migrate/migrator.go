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
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
)

type Migrator struct {
	config     *Config
	dialect    Dialect
	migrations []Migration
}

// New creates a new empty Migrator for the given config & dialect. You must add
// migrations to it before it can be run.
func New(config *Config, dialect Dialect) *Migrator {
	return &Migrator{
		config:     EffectiveConfig(dialect, config),
		dialect:    dialect,
		migrations: []Migration{},
	}
}

func (m *Migrator) WithConfig(config *Config) *Migrator {
	return &Migrator{
		config:     EffectiveConfig(m.dialect, config),
		dialect:    m.dialect,
		migrations: m.migrations,
	}
}

func (m *Migrator) WithDialect(dialect Dialect) *Migrator {
	return &Migrator{
		config:     EffectiveConfig(dialect, m.config),
		dialect:    dialect,
		migrations: m.migrations,
	}
}

func (m *Migrator) HasDialect() bool {
	return m.dialect != nil
}

func (m *Migrator) HasMigrations() bool {
	return len(m.migrations) > 0
}

// NewFromFS is roughly equivalent to New(...).Add("", LoadFS(...)...).
// If you want to apply a prefix, use the individual method calls.
func NewFromFS(
	config *Config,
	dialect Dialect,
	migrationsFS fs.FS,
	filter func(*SQLMigration) bool,
) (*Migrator, error) {
	migrator := New(config, dialect)
	migrations, err := LoadFS(migrationsFS, filter)
	if err != nil {
		return nil, err
	}
	migrator.AddAndSort("", migrations...)
	return migrator, nil
}

// AddAndSort adds the listed migrations, applying the given prefix, to the migrator
// and sorts the resulting list
func (m *Migrator) AddAndSort(prefix string, migrations ...Migration) *Migrator {
	m.migrations = append(m.migrations, WithPrefix(prefix, migrations...)...)
	sortMigrations(m.migrations)
	return m
}

// SortandAppend sorts the listed migrations and then appends them to the
// migrator without further sorting.
func (m *Migrator) SortAndAppend(prefix string, migrations ...Migration) *Migrator {
	sortMigrations(migrations)
	m.migrations = append(m.migrations, WithPrefix(prefix, migrations...)...)
	return m
}

func sortMigrations(migrations []Migration) {
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Name() < migrations[j].Name()
	})
}

func (m *Migrator) Up(
	ctx context.Context,
	db *sqlx.DB,
) error {
	return m.run(ctx, db, true)
}

func (m *Migrator) Down(
	ctx context.Context,
	db *sqlx.DB,
) error {
	return m.run(ctx, db, false)
}

func (m *Migrator) run(
	ctx context.Context,
	db *sqlx.DB,
	direction Direction,
) error {
	if err := m.dialect.Verify(db); err != nil {
		return fmt.Errorf("DB connection is no good: %w", err)
	}

	if err := m.dialect.EnsureMigrationsTable(ctx, db, m.config); err != nil {
		return fmt.Errorf("failed ensuring migrations state table exists: %w", err)
	}

	states, err := m.loadStates(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to load migration states: %w", err)
	}

	todo := m.toRun(direction, states)

	for _, mm := range todo {
		if err = m.runOne(ctx, db, mm, direction); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", mm.Name(), err)
		}
	}

	return nil
}

func (m *Migrator) runOne(
	ctx context.Context,
	db *sqlx.DB,
	mm Migration,
	direction Direction,
) error {
	success := false

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  false,
	})
	if tx != nil {
		defer func() {
			if !success {
				if err := tx.Rollback(); err != nil {
					// TODO: don't panic here
					panic(err)
				}
			}
		}()
	}
	if err != nil {
		return err
	}

	if err = m.dialect.BeforeMigration(ctx, db, tx, mm.Name(), direction); err != nil {
		return err
	}

	if direction == DirectionUp {
		err = mm.Up(ctx, db, tx)
	} else {
		err = mm.Down(ctx, db, tx)
	}
	if err != nil {
		return err
	}

	var query string
	if direction {
		query = m.dialect.InsertMigration(m.config)
	} else {
		query = m.dialect.DeleteMigration(m.config)
	}
	_, err = tx.ExecContext(ctx, query, mm.Name(), time.Now())
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	success = true

	return nil
}

func (m *Migrator) loadStates(
	ctx context.Context,
	db *sqlx.DB,
) ([]*State, error) {
	rows, err := db.QueryxContext(ctx, m.dialect.SelectStates(m.config))
	if rows != nil {
		defer rows.Close()
	}
	if err != nil {
		return nil, err
	}

	ret := make([]*State, 0)

	for rows.Next() {
		s := &State{}
		if err = rows.StructScan(s); err != nil {
			return nil, err
		}
		ret = append(ret, s)
	}

	return ret, nil
}

func (m *Migrator) toRun(
	direction Direction,
	states []*State,
) []Migration {
	stateMap := make(map[string]*State, len(states))
	for _, s := range states {
		stateMap[s.Name] = s
	}

	ret := make([]Migration, 0)
	if direction == DirectionUp {
		// add every migration that isn't recorded in the db in ascending order
		for _, migration := range m.migrations {
			if _, ok := stateMap[migration.Name()]; !ok {
				ret = append(ret, migration)
			}
		}
	} else {
		// add every migration that is recorded in the db in descending order
		for i := len(m.migrations) - 1; i >= 0; i-- {
			migration := m.migrations[i]
			if _, ok := stateMap[migration.Name()]; ok {
				ret = append(ret, migration)
			}
		}
	}

	return ret
}
