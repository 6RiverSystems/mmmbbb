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
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

var nameRegex = regexp.MustCompile(`^(.*)[.-](up|down).sql$`)

func LoadFS(
	migrationsFS fs.FS,
	filter func(*SQLMigration) bool,
) ([]Migration, error) {
	entries := make(map[string]*SQLMigration)
	err := fs.WalkDir(migrationsFS, ".", func(fn string, d fs.DirEntry, err error) error {
		if !d.Type().IsRegular() {
			return nil
		}

		m := nameRegex.FindStringSubmatch(fn)
		if m == nil {
			return fmt.Errorf("Entry '%s' does not look like a migration", fn)
		}
		name, direction := m[1], m[2]
		// for compatibility with db-migrate, where things are always scope/name,
		// where scope may be the empty string, so e.g. "a/b" or "/c", but never "c"
		// or "/a/b"
		if strings.IndexByte(name, '/') < 0 {
			name = "/" + name
		}
		e, ok := entries[name]
		if !ok {
			e = &SQLMigration{name: name}
		}
		content := func() (string, error) {
			data, err := fs.ReadFile(migrationsFS, fn)
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
		if direction == "up" {
			if e.up != nil {
				return fmt.Errorf("Multiple up entries for '%s'", name)
			}
			e.up = content
		} else {
			if e.down != nil {
				return fmt.Errorf("Multiple down entries for '%s'", name)
			}
			e.down = content
		}
		entries[name] = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	// make it a slice
	migrations := make([]Migration, 0, len(entries))
	for _, m := range entries {
		if filter == nil || filter(m) {
			migrations = append(migrations, m)
		}
	}
	return migrations, nil
}
