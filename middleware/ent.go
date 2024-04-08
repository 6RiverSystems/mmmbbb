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

package middleware

import (
	"database/sql"
	"sync"

	"github.com/gin-gonic/gin"

	"go.6river.tech/mmmbbb/db"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/logging"
)

var (
	entClientKeyBase = "ent-client-"
	entTxKeyBase     = "ent-tx-"
)

type EntKey string

var (
	key     EntKey
	keyInit sync.Once
)

func Key() EntKey {
	keyInit.Do(func() {
		key = EntKey(db.GetDefaultDbName())
	})
	return key
}

func WithEntClient(client *ent.Client, name EntKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(entClientKeyBase+string(name), client)
	}
}

func Client(c *gin.Context, name EntKey) *ent.Client {
	// TODO: could have this check for an active transaction and return the
	// transactional client instead in that case
	return c.MustGet(entClientKeyBase + string(name)).(*ent.Client)
}

type TransactionControl func(*gin.Context, *sql.TxOptions) bool

func WithTransaction(
	name EntKey,
	opts *sql.TxOptions,
	controls ...TransactionControl,
) gin.HandlerFunc {
	// due to generics, we can't directly compare T to nil, so we need to track
	// "discard" separately
	finishedTx := false

	txKey := entTxKeyBase + string(name)
	logger := logging.GetLogger("middleware/ent")
	if opts == nil {
		opts = &sql.TxOptions{}
	}
	return func(c *gin.Context) {
		client := Client(c, name)
		// make a copy before we mutate it
		txOpts := *opts
		useTx := true
		for _, control := range controls {
			if !control(c, &txOpts) {
				useTx = false
				break
			}
		}
		if !useTx {
			// don't actually want a transaction, move on to the next handler
			return
		}

		tx, err := client.BeginTx(c.Request.Context(), &txOpts)
		if err != nil {
			// TODO: avoid relying on gin's panic handling
			panic(err)
		}
		c.Set(txKey, tx)
		// TODO: not sure this panic handling is correct
		defer func() {
			// if tx is non-nil, we must have panicked
			if !finishedTx {
				rbErr := tx.Rollback()
				if rbErr != nil {
					// nolint:errcheck // return value here is just a wrapped copy of the input
					c.Error(rbErr)
					// we're about to re-panic, don't overwrite the original
					logger.Err(rbErr).Msg("Failed to rollback during panic")
				}
				finishedTx = true
			}
		}()
		c.Next()
		if len(c.Errors) > 0 || c.IsAborted() {
			rbErr := tx.Rollback()
			if rbErr != nil {
				// nolint:errcheck // return value here is just a wrapped copy of the input
				c.Error(rbErr)
			}
		} else {
			cErr := tx.Commit()
			if cErr != nil {
				// nolint:errcheck // return value here is just a wrapped copy of the input
				c.Error(cErr)
			}
		}
		finishedTx = true
	}
}

func Transaction(c *gin.Context, name EntKey) *ent.Tx {
	return c.MustGet(entTxKeyBase + string(name)).(*ent.Tx)
}
