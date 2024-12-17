// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/migrate"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
)

// Client is the client that holds all ent builders.
type Client struct {
	config
	// Schema is the client for creating, migrating and dropping schema.
	Schema *migrate.Schema
	// Delivery is the client for interacting with the Delivery builders.
	Delivery *DeliveryClient
	// Message is the client for interacting with the Message builders.
	Message *MessageClient
	// Snapshot is the client for interacting with the Snapshot builders.
	Snapshot *SnapshotClient
	// Subscription is the client for interacting with the Subscription builders.
	Subscription *SubscriptionClient
	// Topic is the client for interacting with the Topic builders.
	Topic *TopicClient
}

// NewClient creates a new client configured with the given options.
func NewClient(opts ...Option) *Client {
	client := &Client{config: newConfig(opts...)}
	client.init()
	return client
}

func (c *Client) init() {
	c.Schema = migrate.NewSchema(c.driver)
	c.Delivery = NewDeliveryClient(c.config)
	c.Message = NewMessageClient(c.config)
	c.Snapshot = NewSnapshotClient(c.config)
	c.Subscription = NewSubscriptionClient(c.config)
	c.Topic = NewTopicClient(c.config)
}

type (
	// config is the configuration for the client and its builder.
	config struct {
		// driver used for executing database requests.
		driver dialect.Driver
		// debug enable a debug logging.
		debug bool
		// log used for logging on debug mode.
		log func(...any)
		// hooks to execute on mutations.
		hooks *hooks
		// interceptors to execute on queries.
		inters *inters
	}
	// Option function to configure the client.
	Option func(*config)
)

// newConfig creates a new config for the client.
func newConfig(opts ...Option) config {
	cfg := config{log: log.Println, hooks: &hooks{}, inters: &inters{}}
	cfg.options(opts...)
	return cfg
}

// options applies the options on the config object.
func (c *config) options(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
	if c.debug {
		c.driver = dialect.Debug(c.driver, c.log)
	}
}

// Debug enables debug logging on the ent.Driver.
func Debug() Option {
	return func(c *config) {
		c.debug = true
	}
}

// Log sets the logging function for debug mode.
func Log(fn func(...any)) Option {
	return func(c *config) {
		c.log = fn
	}
}

// Driver configures the client driver.
func Driver(driver dialect.Driver) Option {
	return func(c *config) {
		c.driver = driver
	}
}

// Open opens a database/sql.DB specified by the driver name and
// the data source name, and returns a new client attached to it.
// Optional parameters can be added for configuring the client.
func Open(driverName, dataSourceName string, options ...Option) (*Client, error) {
	switch driverName {
	case dialect.MySQL, dialect.Postgres, dialect.SQLite:
		drv, err := sql.Open(driverName, dataSourceName)
		if err != nil {
			return nil, err
		}
		return NewClient(append(options, Driver(drv))...), nil
	default:
		return nil, fmt.Errorf("unsupported driver: %q", driverName)
	}
}

// ErrTxStarted is returned when trying to start a new transaction from a transactional client.
var ErrTxStarted = errors.New("ent: cannot start a transaction within a transaction")

// Tx returns a new transactional client. The provided context
// is used until the transaction is committed or rolled back.
func (c *Client) Tx(ctx context.Context) (*Tx, error) {
	if _, ok := c.driver.(*txDriver); ok {
		return nil, ErrTxStarted
	}
	tx, err := newTx(ctx, c.driver)
	if err != nil {
		return nil, fmt.Errorf("ent: starting a transaction: %w", err)
	}
	cfg := c.config
	cfg.driver = tx
	return &Tx{
		ctx:          ctx,
		config:       cfg,
		Delivery:     NewDeliveryClient(cfg),
		Message:      NewMessageClient(cfg),
		Snapshot:     NewSnapshotClient(cfg),
		Subscription: NewSubscriptionClient(cfg),
		Topic:        NewTopicClient(cfg),
	}, nil
}

// BeginTx returns a transactional client with specified options.
func (c *Client) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	if _, ok := c.driver.(*txDriver); ok {
		return nil, errors.New("ent: cannot start a transaction within a transaction")
	}
	tx, err := c.driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}).BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("ent: starting a transaction: %w", err)
	}
	cfg := c.config
	cfg.driver = &txDriver{tx: tx, drv: c.driver}
	return &Tx{
		ctx:          ctx,
		config:       cfg,
		Delivery:     NewDeliveryClient(cfg),
		Message:      NewMessageClient(cfg),
		Snapshot:     NewSnapshotClient(cfg),
		Subscription: NewSubscriptionClient(cfg),
		Topic:        NewTopicClient(cfg),
	}, nil
}

// Debug returns a new debug-client. It's used to get verbose logging on specific operations.
//
//	client.Debug().
//		Delivery.
//		Query().
//		Count(ctx)
func (c *Client) Debug() *Client {
	if c.debug {
		return c
	}
	cfg := c.config
	cfg.driver = dialect.Debug(c.driver, c.log)
	client := &Client{config: cfg}
	client.init()
	return client
}

// Close closes the database connection and prevents new queries from starting.
func (c *Client) Close() error {
	return c.driver.Close()
}

// Use adds the mutation hooks to all the entity clients.
// In order to add hooks to a specific client, call: `client.Node.Use(...)`.
func (c *Client) Use(hooks ...Hook) {
	c.Delivery.Use(hooks...)
	c.Message.Use(hooks...)
	c.Snapshot.Use(hooks...)
	c.Subscription.Use(hooks...)
	c.Topic.Use(hooks...)
}

// Intercept adds the query interceptors to all the entity clients.
// In order to add interceptors to a specific client, call: `client.Node.Intercept(...)`.
func (c *Client) Intercept(interceptors ...Interceptor) {
	c.Delivery.Intercept(interceptors...)
	c.Message.Intercept(interceptors...)
	c.Snapshot.Intercept(interceptors...)
	c.Subscription.Intercept(interceptors...)
	c.Topic.Intercept(interceptors...)
}

// Mutate implements the ent.Mutator interface.
func (c *Client) Mutate(ctx context.Context, m Mutation) (Value, error) {
	switch m := m.(type) {
	case *DeliveryMutation:
		return c.Delivery.mutate(ctx, m)
	case *MessageMutation:
		return c.Message.mutate(ctx, m)
	case *SnapshotMutation:
		return c.Snapshot.mutate(ctx, m)
	case *SubscriptionMutation:
		return c.Subscription.mutate(ctx, m)
	case *TopicMutation:
		return c.Topic.mutate(ctx, m)
	default:
		return nil, fmt.Errorf("ent: unknown mutation type %T", m)
	}
}

// DeliveryClient is a client for the Delivery schema.
type DeliveryClient struct {
	config
}

// NewDeliveryClient returns a client for the Delivery from the given config.
func NewDeliveryClient(c config) *DeliveryClient {
	return &DeliveryClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `delivery.Hooks(f(g(h())))`.
func (c *DeliveryClient) Use(hooks ...Hook) {
	c.hooks.Delivery = append(c.hooks.Delivery, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `delivery.Intercept(f(g(h())))`.
func (c *DeliveryClient) Intercept(interceptors ...Interceptor) {
	c.inters.Delivery = append(c.inters.Delivery, interceptors...)
}

// Create returns a builder for creating a Delivery entity.
func (c *DeliveryClient) Create() *DeliveryCreate {
	mutation := newDeliveryMutation(c.config, OpCreate)
	return &DeliveryCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of Delivery entities.
func (c *DeliveryClient) CreateBulk(builders ...*DeliveryCreate) *DeliveryCreateBulk {
	return &DeliveryCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *DeliveryClient) MapCreateBulk(slice any, setFunc func(*DeliveryCreate, int)) *DeliveryCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &DeliveryCreateBulk{err: fmt.Errorf("calling to DeliveryClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*DeliveryCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &DeliveryCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for Delivery.
func (c *DeliveryClient) Update() *DeliveryUpdate {
	mutation := newDeliveryMutation(c.config, OpUpdate)
	return &DeliveryUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *DeliveryClient) UpdateOne(d *Delivery) *DeliveryUpdateOne {
	mutation := newDeliveryMutation(c.config, OpUpdateOne, withDelivery(d))
	return &DeliveryUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *DeliveryClient) UpdateOneID(id uuid.UUID) *DeliveryUpdateOne {
	mutation := newDeliveryMutation(c.config, OpUpdateOne, withDeliveryID(id))
	return &DeliveryUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for Delivery.
func (c *DeliveryClient) Delete() *DeliveryDelete {
	mutation := newDeliveryMutation(c.config, OpDelete)
	return &DeliveryDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *DeliveryClient) DeleteOne(d *Delivery) *DeliveryDeleteOne {
	return c.DeleteOneID(d.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *DeliveryClient) DeleteOneID(id uuid.UUID) *DeliveryDeleteOne {
	builder := c.Delete().Where(delivery.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &DeliveryDeleteOne{builder}
}

// Query returns a query builder for Delivery.
func (c *DeliveryClient) Query() *DeliveryQuery {
	return &DeliveryQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeDelivery},
		inters: c.Interceptors(),
	}
}

// Get returns a Delivery entity by its id.
func (c *DeliveryClient) Get(ctx context.Context, id uuid.UUID) (*Delivery, error) {
	return c.Query().Where(delivery.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *DeliveryClient) GetX(ctx context.Context, id uuid.UUID) *Delivery {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryMessage queries the message edge of a Delivery.
func (c *DeliveryClient) QueryMessage(d *Delivery) *MessageQuery {
	query := (&MessageClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := d.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(delivery.Table, delivery.FieldID, id),
			sqlgraph.To(message.Table, message.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, delivery.MessageTable, delivery.MessageColumn),
		)
		fromV = sqlgraph.Neighbors(d.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QuerySubscription queries the subscription edge of a Delivery.
func (c *DeliveryClient) QuerySubscription(d *Delivery) *SubscriptionQuery {
	query := (&SubscriptionClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := d.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(delivery.Table, delivery.FieldID, id),
			sqlgraph.To(subscription.Table, subscription.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, delivery.SubscriptionTable, delivery.SubscriptionColumn),
		)
		fromV = sqlgraph.Neighbors(d.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryNotBefore queries the notBefore edge of a Delivery.
func (c *DeliveryClient) QueryNotBefore(d *Delivery) *DeliveryQuery {
	query := (&DeliveryClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := d.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(delivery.Table, delivery.FieldID, id),
			sqlgraph.To(delivery.Table, delivery.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, true, delivery.NotBeforeTable, delivery.NotBeforeColumn),
		)
		fromV = sqlgraph.Neighbors(d.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryNextReady queries the nextReady edge of a Delivery.
func (c *DeliveryClient) QueryNextReady(d *Delivery) *DeliveryQuery {
	query := (&DeliveryClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := d.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(delivery.Table, delivery.FieldID, id),
			sqlgraph.To(delivery.Table, delivery.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, false, delivery.NextReadyTable, delivery.NextReadyColumn),
		)
		fromV = sqlgraph.Neighbors(d.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *DeliveryClient) Hooks() []Hook {
	return c.hooks.Delivery
}

// Interceptors returns the client interceptors.
func (c *DeliveryClient) Interceptors() []Interceptor {
	return c.inters.Delivery
}

func (c *DeliveryClient) mutate(ctx context.Context, m *DeliveryMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&DeliveryCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&DeliveryUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&DeliveryUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&DeliveryDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown Delivery mutation op: %q", m.Op())
	}
}

// MessageClient is a client for the Message schema.
type MessageClient struct {
	config
}

// NewMessageClient returns a client for the Message from the given config.
func NewMessageClient(c config) *MessageClient {
	return &MessageClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `message.Hooks(f(g(h())))`.
func (c *MessageClient) Use(hooks ...Hook) {
	c.hooks.Message = append(c.hooks.Message, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `message.Intercept(f(g(h())))`.
func (c *MessageClient) Intercept(interceptors ...Interceptor) {
	c.inters.Message = append(c.inters.Message, interceptors...)
}

// Create returns a builder for creating a Message entity.
func (c *MessageClient) Create() *MessageCreate {
	mutation := newMessageMutation(c.config, OpCreate)
	return &MessageCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of Message entities.
func (c *MessageClient) CreateBulk(builders ...*MessageCreate) *MessageCreateBulk {
	return &MessageCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *MessageClient) MapCreateBulk(slice any, setFunc func(*MessageCreate, int)) *MessageCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &MessageCreateBulk{err: fmt.Errorf("calling to MessageClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*MessageCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &MessageCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for Message.
func (c *MessageClient) Update() *MessageUpdate {
	mutation := newMessageMutation(c.config, OpUpdate)
	return &MessageUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *MessageClient) UpdateOne(m *Message) *MessageUpdateOne {
	mutation := newMessageMutation(c.config, OpUpdateOne, withMessage(m))
	return &MessageUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *MessageClient) UpdateOneID(id uuid.UUID) *MessageUpdateOne {
	mutation := newMessageMutation(c.config, OpUpdateOne, withMessageID(id))
	return &MessageUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for Message.
func (c *MessageClient) Delete() *MessageDelete {
	mutation := newMessageMutation(c.config, OpDelete)
	return &MessageDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *MessageClient) DeleteOne(m *Message) *MessageDeleteOne {
	return c.DeleteOneID(m.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *MessageClient) DeleteOneID(id uuid.UUID) *MessageDeleteOne {
	builder := c.Delete().Where(message.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &MessageDeleteOne{builder}
}

// Query returns a query builder for Message.
func (c *MessageClient) Query() *MessageQuery {
	return &MessageQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeMessage},
		inters: c.Interceptors(),
	}
}

// Get returns a Message entity by its id.
func (c *MessageClient) Get(ctx context.Context, id uuid.UUID) (*Message, error) {
	return c.Query().Where(message.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *MessageClient) GetX(ctx context.Context, id uuid.UUID) *Message {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryDeliveries queries the deliveries edge of a Message.
func (c *MessageClient) QueryDeliveries(m *Message) *DeliveryQuery {
	query := (&DeliveryClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := m.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(message.Table, message.FieldID, id),
			sqlgraph.To(delivery.Table, delivery.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, message.DeliveriesTable, message.DeliveriesColumn),
		)
		fromV = sqlgraph.Neighbors(m.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryTopic queries the topic edge of a Message.
func (c *MessageClient) QueryTopic(m *Message) *TopicQuery {
	query := (&TopicClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := m.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(message.Table, message.FieldID, id),
			sqlgraph.To(topic.Table, topic.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, message.TopicTable, message.TopicColumn),
		)
		fromV = sqlgraph.Neighbors(m.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *MessageClient) Hooks() []Hook {
	return c.hooks.Message
}

// Interceptors returns the client interceptors.
func (c *MessageClient) Interceptors() []Interceptor {
	return c.inters.Message
}

func (c *MessageClient) mutate(ctx context.Context, m *MessageMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&MessageCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&MessageUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&MessageUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&MessageDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown Message mutation op: %q", m.Op())
	}
}

// SnapshotClient is a client for the Snapshot schema.
type SnapshotClient struct {
	config
}

// NewSnapshotClient returns a client for the Snapshot from the given config.
func NewSnapshotClient(c config) *SnapshotClient {
	return &SnapshotClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `snapshot.Hooks(f(g(h())))`.
func (c *SnapshotClient) Use(hooks ...Hook) {
	c.hooks.Snapshot = append(c.hooks.Snapshot, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `snapshot.Intercept(f(g(h())))`.
func (c *SnapshotClient) Intercept(interceptors ...Interceptor) {
	c.inters.Snapshot = append(c.inters.Snapshot, interceptors...)
}

// Create returns a builder for creating a Snapshot entity.
func (c *SnapshotClient) Create() *SnapshotCreate {
	mutation := newSnapshotMutation(c.config, OpCreate)
	return &SnapshotCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of Snapshot entities.
func (c *SnapshotClient) CreateBulk(builders ...*SnapshotCreate) *SnapshotCreateBulk {
	return &SnapshotCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *SnapshotClient) MapCreateBulk(slice any, setFunc func(*SnapshotCreate, int)) *SnapshotCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &SnapshotCreateBulk{err: fmt.Errorf("calling to SnapshotClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*SnapshotCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &SnapshotCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for Snapshot.
func (c *SnapshotClient) Update() *SnapshotUpdate {
	mutation := newSnapshotMutation(c.config, OpUpdate)
	return &SnapshotUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *SnapshotClient) UpdateOne(s *Snapshot) *SnapshotUpdateOne {
	mutation := newSnapshotMutation(c.config, OpUpdateOne, withSnapshot(s))
	return &SnapshotUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *SnapshotClient) UpdateOneID(id uuid.UUID) *SnapshotUpdateOne {
	mutation := newSnapshotMutation(c.config, OpUpdateOne, withSnapshotID(id))
	return &SnapshotUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for Snapshot.
func (c *SnapshotClient) Delete() *SnapshotDelete {
	mutation := newSnapshotMutation(c.config, OpDelete)
	return &SnapshotDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *SnapshotClient) DeleteOne(s *Snapshot) *SnapshotDeleteOne {
	return c.DeleteOneID(s.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *SnapshotClient) DeleteOneID(id uuid.UUID) *SnapshotDeleteOne {
	builder := c.Delete().Where(snapshot.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &SnapshotDeleteOne{builder}
}

// Query returns a query builder for Snapshot.
func (c *SnapshotClient) Query() *SnapshotQuery {
	return &SnapshotQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeSnapshot},
		inters: c.Interceptors(),
	}
}

// Get returns a Snapshot entity by its id.
func (c *SnapshotClient) Get(ctx context.Context, id uuid.UUID) (*Snapshot, error) {
	return c.Query().Where(snapshot.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *SnapshotClient) GetX(ctx context.Context, id uuid.UUID) *Snapshot {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryTopic queries the topic edge of a Snapshot.
func (c *SnapshotClient) QueryTopic(s *Snapshot) *TopicQuery {
	query := (&TopicClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := s.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(snapshot.Table, snapshot.FieldID, id),
			sqlgraph.To(topic.Table, topic.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, snapshot.TopicTable, snapshot.TopicColumn),
		)
		fromV = sqlgraph.Neighbors(s.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *SnapshotClient) Hooks() []Hook {
	return c.hooks.Snapshot
}

// Interceptors returns the client interceptors.
func (c *SnapshotClient) Interceptors() []Interceptor {
	return c.inters.Snapshot
}

func (c *SnapshotClient) mutate(ctx context.Context, m *SnapshotMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&SnapshotCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&SnapshotUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&SnapshotUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&SnapshotDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown Snapshot mutation op: %q", m.Op())
	}
}

// SubscriptionClient is a client for the Subscription schema.
type SubscriptionClient struct {
	config
}

// NewSubscriptionClient returns a client for the Subscription from the given config.
func NewSubscriptionClient(c config) *SubscriptionClient {
	return &SubscriptionClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `subscription.Hooks(f(g(h())))`.
func (c *SubscriptionClient) Use(hooks ...Hook) {
	c.hooks.Subscription = append(c.hooks.Subscription, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `subscription.Intercept(f(g(h())))`.
func (c *SubscriptionClient) Intercept(interceptors ...Interceptor) {
	c.inters.Subscription = append(c.inters.Subscription, interceptors...)
}

// Create returns a builder for creating a Subscription entity.
func (c *SubscriptionClient) Create() *SubscriptionCreate {
	mutation := newSubscriptionMutation(c.config, OpCreate)
	return &SubscriptionCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of Subscription entities.
func (c *SubscriptionClient) CreateBulk(builders ...*SubscriptionCreate) *SubscriptionCreateBulk {
	return &SubscriptionCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *SubscriptionClient) MapCreateBulk(slice any, setFunc func(*SubscriptionCreate, int)) *SubscriptionCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &SubscriptionCreateBulk{err: fmt.Errorf("calling to SubscriptionClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*SubscriptionCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &SubscriptionCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for Subscription.
func (c *SubscriptionClient) Update() *SubscriptionUpdate {
	mutation := newSubscriptionMutation(c.config, OpUpdate)
	return &SubscriptionUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *SubscriptionClient) UpdateOne(s *Subscription) *SubscriptionUpdateOne {
	mutation := newSubscriptionMutation(c.config, OpUpdateOne, withSubscription(s))
	return &SubscriptionUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *SubscriptionClient) UpdateOneID(id uuid.UUID) *SubscriptionUpdateOne {
	mutation := newSubscriptionMutation(c.config, OpUpdateOne, withSubscriptionID(id))
	return &SubscriptionUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for Subscription.
func (c *SubscriptionClient) Delete() *SubscriptionDelete {
	mutation := newSubscriptionMutation(c.config, OpDelete)
	return &SubscriptionDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *SubscriptionClient) DeleteOne(s *Subscription) *SubscriptionDeleteOne {
	return c.DeleteOneID(s.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *SubscriptionClient) DeleteOneID(id uuid.UUID) *SubscriptionDeleteOne {
	builder := c.Delete().Where(subscription.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &SubscriptionDeleteOne{builder}
}

// Query returns a query builder for Subscription.
func (c *SubscriptionClient) Query() *SubscriptionQuery {
	return &SubscriptionQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeSubscription},
		inters: c.Interceptors(),
	}
}

// Get returns a Subscription entity by its id.
func (c *SubscriptionClient) Get(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	return c.Query().Where(subscription.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *SubscriptionClient) GetX(ctx context.Context, id uuid.UUID) *Subscription {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QueryTopic queries the topic edge of a Subscription.
func (c *SubscriptionClient) QueryTopic(s *Subscription) *TopicQuery {
	query := (&TopicClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := s.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(subscription.Table, subscription.FieldID, id),
			sqlgraph.To(topic.Table, topic.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, subscription.TopicTable, subscription.TopicColumn),
		)
		fromV = sqlgraph.Neighbors(s.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryDeliveries queries the deliveries edge of a Subscription.
func (c *SubscriptionClient) QueryDeliveries(s *Subscription) *DeliveryQuery {
	query := (&DeliveryClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := s.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(subscription.Table, subscription.FieldID, id),
			sqlgraph.To(delivery.Table, delivery.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, subscription.DeliveriesTable, subscription.DeliveriesColumn),
		)
		fromV = sqlgraph.Neighbors(s.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryDeadLetterTopic queries the deadLetterTopic edge of a Subscription.
func (c *SubscriptionClient) QueryDeadLetterTopic(s *Subscription) *TopicQuery {
	query := (&TopicClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := s.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(subscription.Table, subscription.FieldID, id),
			sqlgraph.To(topic.Table, topic.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, subscription.DeadLetterTopicTable, subscription.DeadLetterTopicColumn),
		)
		fromV = sqlgraph.Neighbors(s.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *SubscriptionClient) Hooks() []Hook {
	hooks := c.hooks.Subscription
	return append(hooks[:len(hooks):len(hooks)], subscription.Hooks[:]...)
}

// Interceptors returns the client interceptors.
func (c *SubscriptionClient) Interceptors() []Interceptor {
	return c.inters.Subscription
}

func (c *SubscriptionClient) mutate(ctx context.Context, m *SubscriptionMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&SubscriptionCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&SubscriptionUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&SubscriptionUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&SubscriptionDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown Subscription mutation op: %q", m.Op())
	}
}

// TopicClient is a client for the Topic schema.
type TopicClient struct {
	config
}

// NewTopicClient returns a client for the Topic from the given config.
func NewTopicClient(c config) *TopicClient {
	return &TopicClient{config: c}
}

// Use adds a list of mutation hooks to the hooks stack.
// A call to `Use(f, g, h)` equals to `topic.Hooks(f(g(h())))`.
func (c *TopicClient) Use(hooks ...Hook) {
	c.hooks.Topic = append(c.hooks.Topic, hooks...)
}

// Intercept adds a list of query interceptors to the interceptors stack.
// A call to `Intercept(f, g, h)` equals to `topic.Intercept(f(g(h())))`.
func (c *TopicClient) Intercept(interceptors ...Interceptor) {
	c.inters.Topic = append(c.inters.Topic, interceptors...)
}

// Create returns a builder for creating a Topic entity.
func (c *TopicClient) Create() *TopicCreate {
	mutation := newTopicMutation(c.config, OpCreate)
	return &TopicCreate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// CreateBulk returns a builder for creating a bulk of Topic entities.
func (c *TopicClient) CreateBulk(builders ...*TopicCreate) *TopicCreateBulk {
	return &TopicCreateBulk{config: c.config, builders: builders}
}

// MapCreateBulk creates a bulk creation builder from the given slice. For each item in the slice, the function creates
// a builder and applies setFunc on it.
func (c *TopicClient) MapCreateBulk(slice any, setFunc func(*TopicCreate, int)) *TopicCreateBulk {
	rv := reflect.ValueOf(slice)
	if rv.Kind() != reflect.Slice {
		return &TopicCreateBulk{err: fmt.Errorf("calling to TopicClient.MapCreateBulk with wrong type %T, need slice", slice)}
	}
	builders := make([]*TopicCreate, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		builders[i] = c.Create()
		setFunc(builders[i], i)
	}
	return &TopicCreateBulk{config: c.config, builders: builders}
}

// Update returns an update builder for Topic.
func (c *TopicClient) Update() *TopicUpdate {
	mutation := newTopicMutation(c.config, OpUpdate)
	return &TopicUpdate{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOne returns an update builder for the given entity.
func (c *TopicClient) UpdateOne(t *Topic) *TopicUpdateOne {
	mutation := newTopicMutation(c.config, OpUpdateOne, withTopic(t))
	return &TopicUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// UpdateOneID returns an update builder for the given id.
func (c *TopicClient) UpdateOneID(id uuid.UUID) *TopicUpdateOne {
	mutation := newTopicMutation(c.config, OpUpdateOne, withTopicID(id))
	return &TopicUpdateOne{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// Delete returns a delete builder for Topic.
func (c *TopicClient) Delete() *TopicDelete {
	mutation := newTopicMutation(c.config, OpDelete)
	return &TopicDelete{config: c.config, hooks: c.Hooks(), mutation: mutation}
}

// DeleteOne returns a builder for deleting the given entity.
func (c *TopicClient) DeleteOne(t *Topic) *TopicDeleteOne {
	return c.DeleteOneID(t.ID)
}

// DeleteOneID returns a builder for deleting the given entity by its id.
func (c *TopicClient) DeleteOneID(id uuid.UUID) *TopicDeleteOne {
	builder := c.Delete().Where(topic.ID(id))
	builder.mutation.id = &id
	builder.mutation.op = OpDeleteOne
	return &TopicDeleteOne{builder}
}

// Query returns a query builder for Topic.
func (c *TopicClient) Query() *TopicQuery {
	return &TopicQuery{
		config: c.config,
		ctx:    &QueryContext{Type: TypeTopic},
		inters: c.Interceptors(),
	}
}

// Get returns a Topic entity by its id.
func (c *TopicClient) Get(ctx context.Context, id uuid.UUID) (*Topic, error) {
	return c.Query().Where(topic.ID(id)).Only(ctx)
}

// GetX is like Get, but panics if an error occurs.
func (c *TopicClient) GetX(ctx context.Context, id uuid.UUID) *Topic {
	obj, err := c.Get(ctx, id)
	if err != nil {
		panic(err)
	}
	return obj
}

// QuerySubscriptions queries the subscriptions edge of a Topic.
func (c *TopicClient) QuerySubscriptions(t *Topic) *SubscriptionQuery {
	query := (&SubscriptionClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := t.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(topic.Table, topic.FieldID, id),
			sqlgraph.To(subscription.Table, subscription.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, topic.SubscriptionsTable, topic.SubscriptionsColumn),
		)
		fromV = sqlgraph.Neighbors(t.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// QueryMessages queries the messages edge of a Topic.
func (c *TopicClient) QueryMessages(t *Topic) *MessageQuery {
	query := (&MessageClient{config: c.config}).Query()
	query.path = func(context.Context) (fromV *sql.Selector, _ error) {
		id := t.ID
		step := sqlgraph.NewStep(
			sqlgraph.From(topic.Table, topic.FieldID, id),
			sqlgraph.To(message.Table, message.FieldID),
			sqlgraph.Edge(sqlgraph.O2M, true, topic.MessagesTable, topic.MessagesColumn),
		)
		fromV = sqlgraph.Neighbors(t.driver.Dialect(), step)
		return fromV, nil
	}
	return query
}

// Hooks returns the client hooks.
func (c *TopicClient) Hooks() []Hook {
	hooks := c.hooks.Topic
	return append(hooks[:len(hooks):len(hooks)], topic.Hooks[:]...)
}

// Interceptors returns the client interceptors.
func (c *TopicClient) Interceptors() []Interceptor {
	return c.inters.Topic
}

func (c *TopicClient) mutate(ctx context.Context, m *TopicMutation) (Value, error) {
	switch m.Op() {
	case OpCreate:
		return (&TopicCreate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdate:
		return (&TopicUpdate{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpUpdateOne:
		return (&TopicUpdateOne{config: c.config, hooks: c.Hooks(), mutation: m}).Save(ctx)
	case OpDelete, OpDeleteOne:
		return (&TopicDelete{config: c.config, hooks: c.Hooks(), mutation: m}).Exec(ctx)
	default:
		return nil, fmt.Errorf("ent: unknown Topic mutation op: %q", m.Op())
	}
}

// hooks and interceptors per client, for fast access.
type (
	hooks struct {
		Delivery, Message, Snapshot, Subscription, Topic []ent.Hook
	}
	inters struct {
		Delivery, Message, Snapshot, Subscription, Topic []ent.Interceptor
	}
)
