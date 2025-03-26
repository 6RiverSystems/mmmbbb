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
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Direction bool

const (
	DirectionUp   Direction = true
	DirectionDown Direction = false
)

func (d Direction) String() string {
	if d {
		return "up"
	}
	return "down"
}

type ContentGetter func() (string, error)

// Migration represents a single migration that can be run to do a single atomic
// step of changing database schemas. Each migration will be run in an
// independent transaction.
type Migration interface {
	// Name is used to track whether the migration has been run, by recording the
	// names of migrations that have been executed in a table.
	Name() string
	// Up runs the migration in the up migration, e.g. creating new tables, to
	// upgrade the schema to the new state. The migration MUST perform all changes
	// within the given transaction, and MUST NOT terminate the transaction.
	Up(context.Context, *sqlx.DB, *sqlx.Tx) error
	// Down runs the migration in the down migration, e.g. dropping tables, to
	// downgrade the schema to the old state. The migration MUST perform all
	// changes within the given transaction, and MUST NOT terminate the
	// transaction.
	Down(context.Context, *sqlx.DB, *sqlx.Tx) error
}

type SQLMigration struct {
	name string
	up   ContentGetter
	down ContentGetter
}

// SQLMigration implements Migration
var _ Migration = &SQLMigration{}

func (m *SQLMigration) Name() string { return m.name }

func (m *SQLMigration) Contents(up Direction) (string, error) {
	var getter ContentGetter
	if up {
		getter = m.up
	} else {
		getter = m.down
	}
	if getter == nil {
		return "", nil
	}
	return getter()
}

func (m *SQLMigration) Up(ctx context.Context, _ *sqlx.DB, tx *sqlx.Tx) error {
	return m.run(ctx, tx, true)
}

func (m *SQLMigration) Down(ctx context.Context, _ *sqlx.DB, tx *sqlx.Tx) error {
	return m.run(ctx, tx, false)
}

func (m *SQLMigration) run(ctx context.Context, tx *sqlx.Tx, direction Direction) error {
	contents, err := m.Contents(direction)
	if err != nil {
		return err
	}
	// if there is no migration script, we cannot do this step
	// this is meant to catch "no going back" migrations where once you "up",
	// there is no (safe) way to go back down again.
	if contents == "" {
		return fmt.Errorf("no script to %s migration %s", direction, m.name)
	}

	_, err = tx.ExecContext(ctx, contents)
	return err
}

// FromSQL generates a SQL Migration based on fixed SQL scripts.
func FromSQL(name, up, down string) *SQLMigration {
	ret := &SQLMigration{name: name}
	if up != "" {
		ret.up = func() (string, error) { return up, nil }
	}
	if down != "" {
		ret.up = func() (string, error) { return down, nil }
	}
	return ret
}

// FromContent generates a SQL Migration from dynamic SQL script getters.
func FromContent(name string, up, down ContentGetter) *SQLMigration {
	return &SQLMigration{
		name: name,
		up:   up,
		down: down,
	}
}

type renamedMigration struct {
	name string
	Migration
}

var _ Migration = &renamedMigration{}

func (m *renamedMigration) Name() string { return m.name }

// WithPrefix returns either the input slice if prefix is the empty string or
// migrations is an empty slice, or else a copy of it with prefix applied to the
// names.
func WithPrefix(prefix string, migrations ...Migration) []Migration {
	if prefix == "" || len(migrations) == 0 {
		return migrations
	}
	ret := make([]Migration, len(migrations))
	for i, m := range migrations {
		ret[i] = &renamedMigration{prefix + m.Name(), m}
	}
	return ret
}
