package ent

import (
	"context"
	"database/sql"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/pkg/errors"

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
		panic(errors.Errorf("Invalid entity name '%s'", name))
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
				finalErr = errors.Wrapf(finalErr, "%s Failed: %s During: %s", op, err.Error(), finalErr.Error())
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
		panic(errors.Errorf("Unable to find DB from %T", driver))
	}
}

func (tx *Tx) DialectTx() dialect.Tx {
	return tx.config.driver.(*txDriver).tx
}

func (tx *Tx) DBTx() *sql.Tx {
	return tx.DialectTx().(*entsql.Tx).Tx.(*sql.Tx)
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
