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
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// reference: https://www.postgresql.org/docs/9.6/errcodes-appendix.html

type PostgreSQLErrorCode string

var (
	SerializationFailure PostgreSQLErrorCode = "40001"
	// InvalidCatalogName often indicates attempting to connect to a database that
	// does not exist
	InvalidCatalogName PostgreSQLErrorCode = "3D000"
	// CannotConnectNow often indicates the server is still starting up
	CannotConnectNow PostgreSQLErrorCode = "57P03"
	DeadlockDetected PostgreSQLErrorCode = "40P01"
)

func IsPostgreSQLErrorCode(err error, code PostgreSQLErrorCode) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	match := errors.As(err, &pgErr) && pgErr.Code == string(code)
	return pgErr, match
}

func RetryOnErrorCode(
	code PostgreSQLErrorCode,
	codes ...PostgreSQLErrorCode,
) func(context.Context, error) bool {
	allCodes := append([]PostgreSQLErrorCode{code}, codes...)
	return func(ctx context.Context, err error) bool {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			for _, c := range allCodes {
				if pgErr.Code == string(c) {
					return true
				}
			}
		}
		return false
	}
}
