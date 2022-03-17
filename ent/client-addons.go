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

package ent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"

	entcommon "go.6river.tech/gosix/ent"
)

// custom add-ons to the Client type for use in our environment

// BeginTxGeneric implements ginmiddleware.EntClient
func (c *Client) BeginTxGeneric(ctx context.Context, opts *sql.TxOptions) (entcommon.EntTx, error) {
	return c.BeginTx(ctx, opts)
}

func (c *Client) EntityClient(name string) entcommon.EntityClient {
	switch name {
	case "Topic":
		return c.Topic
	case "Subscription":
		return c.Subscription
	case "Message":
		return c.Message
	case "Delivery":
		return c.Delivery
	default:
		panic(fmt.Errorf("Invalid entity name '%s'", name))
	}
}

// DoTx wraps inner in a transaction, which will be committed if it returns nil
// or rolled back if it returns an error
func (c *Client) DoTx(ctx context.Context, opts *sql.TxOptions, inner func(tx *Tx) error) (finalErr error) {
	tx, finalErr := c.BeginTx(ctx, opts)
	if finalErr != nil {
		return
	}
	success := false
	defer func() {
		var err error
		var op string
		if !success {
			err = tx.Rollback()
			op = "Rollback"
		} else {
			err = tx.Commit()
			op = "Commit"
		}
		if err != nil {
			// if we get a context cancellation, we may also expect to often get an
			// ErrTxDone, due to the db package racing with us to rollback the
			// transaction
			if errors.Is(err, sql.ErrTxDone) && (errors.Is(finalErr, context.Canceled) || errors.Is(finalErr, context.DeadlineExceeded)) {
				// leave finalErr as-is, ignore the sql error
			} else if finalErr == nil {
				finalErr = err
			} else {
				finalErr = fmt.Errorf("%s Failed: %s During: %w", op, err.Error(), finalErr)
			}
		}
	}()

	finalErr = inner(tx)
	if finalErr == nil {
		success = true
	}
	return
}

// DoCtxTx is a wrapper for DoTx, for handlers that take the context argument.
// This is particularly useful for actions.Action.Execute implementations
func (c *Client) DoCtxTx(ctx context.Context, opts *sql.TxOptions, inner func(ctx context.Context, tx *Tx) error) error {
	return c.DoTx(ctx, opts, func(tx *Tx) error { return inner(ctx, tx) })
}

func (c *Client) DoCtxTxRetry(
	ctx context.Context,
	opts *sql.TxOptions,
	inner func(ctx context.Context, tx *Tx) error,
	retry func(ctx context.Context, err error) bool,
) error {
	for {
		err := c.DoTx(ctx, opts, func(tx *Tx) error { return inner(ctx, tx) })
		if err == nil || !retry(ctx, err) {
			return err
		}
	}
}

func (c *Client) GetSchema() entcommon.EntClientSchema {
	return c.Schema
}

func (c *Client) Dialect() string {
	return c.driver.Dialect()
}

func (c *Client) DB() *sql.DB {
	return DriverDB(c.driver)
}

func DriverDB(driver dialect.Driver) *sql.DB {
	switch d := driver.(type) {
	case *entsql.Driver:
		return d.Conn.ExecQuerier.(*sql.DB)
	case *dialect.DebugDriver:
		return DriverDB(d.Driver)
	default:
		panic(fmt.Errorf("Unable to find DB from %T", driver))
	}
}

func (tx *Tx) DialectTx() dialect.Tx {
	return tx.config.driver.(*txDriver).tx
}

func (tx *Tx) DBTx() *sql.Tx {
	etx := tx.DialectTx()
	for {
		switch ttx := etx.(type) {
		case *entsql.Tx:
			return ttx.Tx.(*sql.Tx)
		case *dialect.DebugTx:
			etx = ttx.Tx
		default:
			panic(fmt.Errorf("Unrecognized dialect.Tx type %T", etx))
		}
	}
}

func (tx *Tx) Dialect() string {
	return tx.driver.Dialect()
}

func (c *TopicClient) CreateEntity() entcommon.EntityCreate {
	return c.Create()
}

func (cc *TopicCreate) EntityMutation() ent.Mutation {
	return cc.Mutation()
}

func (cc *TopicCreate) SaveEntity(ctx context.Context) (interface{}, error) {
	return cc.Save(ctx)
}

func (c *SubscriptionClient) CreateEntity() entcommon.EntityCreate {
	return c.Create()
}

func (cc *SubscriptionCreate) EntityMutation() ent.Mutation {
	return cc.Mutation()
}

func (cc *SubscriptionCreate) SaveEntity(ctx context.Context) (interface{}, error) {
	return cc.Save(ctx)
}

func (c *MessageClient) CreateEntity() entcommon.EntityCreate {
	return c.Create()
}

func (cc *MessageCreate) EntityMutation() ent.Mutation {
	return cc.Mutation()
}

func (cc *MessageCreate) SaveEntity(ctx context.Context) (interface{}, error) {
	return cc.Save(ctx)
}

func (c *DeliveryClient) CreateEntity() entcommon.EntityCreate {
	return c.Create()
}

func (cc *DeliveryCreate) EntityMutation() ent.Mutation {
	return cc.Mutation()
}

func (cc *DeliveryCreate) SaveEntity(ctx context.Context) (interface{}, error) {
	return cc.Save(ctx)
}
