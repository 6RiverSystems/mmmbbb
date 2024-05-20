// Copyright (c) 2022 6 River Systems
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
	"fmt"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"
)

// in non-CGO mode, use the modernc.org/sqlite driver
const SQLiteDriverName = "sqlite"

func SQLiteDSN(filename string, fileScheme, memory bool) string {
	if strings.HasPrefix(filename, "/") {
		// sqlite:relativepath or sqlite:///some/abs/path
		filename = "//" + filename
	}
	if !strings.HasSuffix(filename, ".sqlite3") {
		filename = filename + ".sqlite3"
	}
	scheme := "sqlite"
	if fileScheme {
		scheme = "file"
	}
	// WARNING: keep these in sync with the CGO version
	q := url.Values{
		"_pragma": []string{
			"foreign_keys(1)",
			"journal_mode(wal)",
			"busy_timeout(10000)",
		},
		// ref: https://gitlab.com/cznic/sqlite/-/issues/92
		// we need BEGIN IMMEDIATE for several use cases to work
		"_txlock": []string{"immediate"},
	}
	if memory {
		filename = ":memory:"
		// memory mode needs either shared cache, or single connection. shared cache
		// doesn't play nice and results in lots of unfixable "table is locked"
		// errors, so instead rely on `Open` to set the max conns appropriately
		q.Set("mode", "memory")
		// q.Set("cache", "shared")

		// even so, memory mode is not safe, as a transaction associated with a
		// canceled context will cause the connection to be forcibly closed and thus
		// the whole database to be lost, schema and all:
		// https://github.com/mattn/go-sqlite3/issues/923
	}
	return fmt.Sprintf("%s:%s?%s", scheme, filename, q.Encode())
}
