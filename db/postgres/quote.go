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

package postgres

import (
	"encoding/hex"
	"strings"
)

// copy-pasta from pgx internals, see
// https://github.com/jackc/pgx/blob/master/conn.go#L341 and
// https://github.com/jackc/pgx/issues/202
func QuoteIdentifier(s string) string {
	return `"` + strings.Replace(s, `"`, `""`, -1) + `"`
}

// copy-pasta from pgx internals, see
// https://github.com/jackc/pgx/blob/master/internal/sanitize/sanitize.go
func QuoteString(str string) string {
	return "'" + strings.Replace(str, "'", "''", -1) + "'"
}

// copy-pasta from pgx internals, see
// https://github.com/jackc/pgx/blob/master/internal/sanitize/sanitize.go
func QuoteBytes(buf []byte) string {
	return `'\x` + hex.EncodeToString(buf) + "'"
}
