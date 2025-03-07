// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"fmt"
	"math"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/predicate"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/topic"
)

// SnapshotQuery is the builder for querying Snapshot entities.
type SnapshotQuery struct {
	config
	ctx        *QueryContext
	order      []snapshot.OrderOption
	inters     []Interceptor
	predicates []predicate.Snapshot
	withTopic  *TopicQuery
	modifiers  []func(*sql.Selector)
	// intermediate query (i.e. traversal path).
	sql  *sql.Selector
	path func(context.Context) (*sql.Selector, error)
}

// Where adds a new predicate for the SnapshotQuery builder.
func (sq *SnapshotQuery) Where(ps ...predicate.Snapshot) *SnapshotQuery {
	sq.predicates = append(sq.predicates, ps...)
	return sq
}

// Limit the number of records to be returned by this query.
func (sq *SnapshotQuery) Limit(limit int) *SnapshotQuery {
	sq.ctx.Limit = &limit
	return sq
}

// Offset to start from.
func (sq *SnapshotQuery) Offset(offset int) *SnapshotQuery {
	sq.ctx.Offset = &offset
	return sq
}

// Unique configures the query builder to filter duplicate records on query.
// By default, unique is set to true, and can be disabled using this method.
func (sq *SnapshotQuery) Unique(unique bool) *SnapshotQuery {
	sq.ctx.Unique = &unique
	return sq
}

// Order specifies how the records should be ordered.
func (sq *SnapshotQuery) Order(o ...snapshot.OrderOption) *SnapshotQuery {
	sq.order = append(sq.order, o...)
	return sq
}

// QueryTopic chains the current query on the "topic" edge.
func (sq *SnapshotQuery) QueryTopic() *TopicQuery {
	query := (&TopicClient{config: sq.config}).Query()
	query.path = func(ctx context.Context) (fromU *sql.Selector, err error) {
		if err := sq.prepareQuery(ctx); err != nil {
			return nil, err
		}
		selector := sq.sqlQuery(ctx)
		if err := selector.Err(); err != nil {
			return nil, err
		}
		step := sqlgraph.NewStep(
			sqlgraph.From(snapshot.Table, snapshot.FieldID, selector),
			sqlgraph.To(topic.Table, topic.FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, snapshot.TopicTable, snapshot.TopicColumn),
		)
		fromU = sqlgraph.SetNeighbors(sq.driver.Dialect(), step)
		return fromU, nil
	}
	return query
}

// First returns the first Snapshot entity from the query.
// Returns a *NotFoundError when no Snapshot was found.
func (sq *SnapshotQuery) First(ctx context.Context) (*Snapshot, error) {
	nodes, err := sq.Limit(1).All(setContextOp(ctx, sq.ctx, ent.OpQueryFirst))
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, &NotFoundError{snapshot.Label}
	}
	return nodes[0], nil
}

// FirstX is like First, but panics if an error occurs.
func (sq *SnapshotQuery) FirstX(ctx context.Context) *Snapshot {
	node, err := sq.First(ctx)
	if err != nil && !IsNotFound(err) {
		panic(err)
	}
	return node
}

// FirstID returns the first Snapshot ID from the query.
// Returns a *NotFoundError when no Snapshot ID was found.
func (sq *SnapshotQuery) FirstID(ctx context.Context) (id uuid.UUID, err error) {
	var ids []uuid.UUID
	if ids, err = sq.Limit(1).IDs(setContextOp(ctx, sq.ctx, ent.OpQueryFirstID)); err != nil {
		return
	}
	if len(ids) == 0 {
		err = &NotFoundError{snapshot.Label}
		return
	}
	return ids[0], nil
}

// FirstIDX is like FirstID, but panics if an error occurs.
func (sq *SnapshotQuery) FirstIDX(ctx context.Context) uuid.UUID {
	id, err := sq.FirstID(ctx)
	if err != nil && !IsNotFound(err) {
		panic(err)
	}
	return id
}

// Only returns a single Snapshot entity found by the query, ensuring it only returns one.
// Returns a *NotSingularError when more than one Snapshot entity is found.
// Returns a *NotFoundError when no Snapshot entities are found.
func (sq *SnapshotQuery) Only(ctx context.Context) (*Snapshot, error) {
	nodes, err := sq.Limit(2).All(setContextOp(ctx, sq.ctx, ent.OpQueryOnly))
	if err != nil {
		return nil, err
	}
	switch len(nodes) {
	case 1:
		return nodes[0], nil
	case 0:
		return nil, &NotFoundError{snapshot.Label}
	default:
		return nil, &NotSingularError{snapshot.Label}
	}
}

// OnlyX is like Only, but panics if an error occurs.
func (sq *SnapshotQuery) OnlyX(ctx context.Context) *Snapshot {
	node, err := sq.Only(ctx)
	if err != nil {
		panic(err)
	}
	return node
}

// OnlyID is like Only, but returns the only Snapshot ID in the query.
// Returns a *NotSingularError when more than one Snapshot ID is found.
// Returns a *NotFoundError when no entities are found.
func (sq *SnapshotQuery) OnlyID(ctx context.Context) (id uuid.UUID, err error) {
	var ids []uuid.UUID
	if ids, err = sq.Limit(2).IDs(setContextOp(ctx, sq.ctx, ent.OpQueryOnlyID)); err != nil {
		return
	}
	switch len(ids) {
	case 1:
		id = ids[0]
	case 0:
		err = &NotFoundError{snapshot.Label}
	default:
		err = &NotSingularError{snapshot.Label}
	}
	return
}

// OnlyIDX is like OnlyID, but panics if an error occurs.
func (sq *SnapshotQuery) OnlyIDX(ctx context.Context) uuid.UUID {
	id, err := sq.OnlyID(ctx)
	if err != nil {
		panic(err)
	}
	return id
}

// All executes the query and returns a list of Snapshots.
func (sq *SnapshotQuery) All(ctx context.Context) ([]*Snapshot, error) {
	ctx = setContextOp(ctx, sq.ctx, ent.OpQueryAll)
	if err := sq.prepareQuery(ctx); err != nil {
		return nil, err
	}
	qr := querierAll[[]*Snapshot, *SnapshotQuery]()
	return withInterceptors[[]*Snapshot](ctx, sq, qr, sq.inters)
}

// AllX is like All, but panics if an error occurs.
func (sq *SnapshotQuery) AllX(ctx context.Context) []*Snapshot {
	nodes, err := sq.All(ctx)
	if err != nil {
		panic(err)
	}
	return nodes
}

// IDs executes the query and returns a list of Snapshot IDs.
func (sq *SnapshotQuery) IDs(ctx context.Context) (ids []uuid.UUID, err error) {
	if sq.ctx.Unique == nil && sq.path != nil {
		sq.Unique(true)
	}
	ctx = setContextOp(ctx, sq.ctx, ent.OpQueryIDs)
	if err = sq.Select(snapshot.FieldID).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// IDsX is like IDs, but panics if an error occurs.
func (sq *SnapshotQuery) IDsX(ctx context.Context) []uuid.UUID {
	ids, err := sq.IDs(ctx)
	if err != nil {
		panic(err)
	}
	return ids
}

// Count returns the count of the given query.
func (sq *SnapshotQuery) Count(ctx context.Context) (int, error) {
	ctx = setContextOp(ctx, sq.ctx, ent.OpQueryCount)
	if err := sq.prepareQuery(ctx); err != nil {
		return 0, err
	}
	return withInterceptors[int](ctx, sq, querierCount[*SnapshotQuery](), sq.inters)
}

// CountX is like Count, but panics if an error occurs.
func (sq *SnapshotQuery) CountX(ctx context.Context) int {
	count, err := sq.Count(ctx)
	if err != nil {
		panic(err)
	}
	return count
}

// Exist returns true if the query has elements in the graph.
func (sq *SnapshotQuery) Exist(ctx context.Context) (bool, error) {
	ctx = setContextOp(ctx, sq.ctx, ent.OpQueryExist)
	switch _, err := sq.FirstID(ctx); {
	case IsNotFound(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("ent: check existence: %w", err)
	default:
		return true, nil
	}
}

// ExistX is like Exist, but panics if an error occurs.
func (sq *SnapshotQuery) ExistX(ctx context.Context) bool {
	exist, err := sq.Exist(ctx)
	if err != nil {
		panic(err)
	}
	return exist
}

// Clone returns a duplicate of the SnapshotQuery builder, including all associated steps. It can be
// used to prepare common query builders and use them differently after the clone is made.
func (sq *SnapshotQuery) Clone() *SnapshotQuery {
	if sq == nil {
		return nil
	}
	return &SnapshotQuery{
		config:     sq.config,
		ctx:        sq.ctx.Clone(),
		order:      append([]snapshot.OrderOption{}, sq.order...),
		inters:     append([]Interceptor{}, sq.inters...),
		predicates: append([]predicate.Snapshot{}, sq.predicates...),
		withTopic:  sq.withTopic.Clone(),
		// clone intermediate query.
		sql:  sq.sql.Clone(),
		path: sq.path,
	}
}

// WithTopic tells the query-builder to eager-load the nodes that are connected to
// the "topic" edge. The optional arguments are used to configure the query builder of the edge.
func (sq *SnapshotQuery) WithTopic(opts ...func(*TopicQuery)) *SnapshotQuery {
	query := (&TopicClient{config: sq.config}).Query()
	for _, opt := range opts {
		opt(query)
	}
	sq.withTopic = query
	return sq
}

// GroupBy is used to group vertices by one or more fields/columns.
// It is often used with aggregate functions, like: count, max, mean, min, sum.
//
// Example:
//
//	var v []struct {
//		TopicID uuid.UUID `json:"topicID,omitempty"`
//		Count int `json:"count,omitempty"`
//	}
//
//	client.Snapshot.Query().
//		GroupBy(snapshot.FieldTopicID).
//		Aggregate(ent.Count()).
//		Scan(ctx, &v)
func (sq *SnapshotQuery) GroupBy(field string, fields ...string) *SnapshotGroupBy {
	sq.ctx.Fields = append([]string{field}, fields...)
	grbuild := &SnapshotGroupBy{build: sq}
	grbuild.flds = &sq.ctx.Fields
	grbuild.label = snapshot.Label
	grbuild.scan = grbuild.Scan
	return grbuild
}

// Select allows the selection one or more fields/columns for the given query,
// instead of selecting all fields in the entity.
//
// Example:
//
//	var v []struct {
//		TopicID uuid.UUID `json:"topicID,omitempty"`
//	}
//
//	client.Snapshot.Query().
//		Select(snapshot.FieldTopicID).
//		Scan(ctx, &v)
func (sq *SnapshotQuery) Select(fields ...string) *SnapshotSelect {
	sq.ctx.Fields = append(sq.ctx.Fields, fields...)
	sbuild := &SnapshotSelect{SnapshotQuery: sq}
	sbuild.label = snapshot.Label
	sbuild.flds, sbuild.scan = &sq.ctx.Fields, sbuild.Scan
	return sbuild
}

// Aggregate returns a SnapshotSelect configured with the given aggregations.
func (sq *SnapshotQuery) Aggregate(fns ...AggregateFunc) *SnapshotSelect {
	return sq.Select().Aggregate(fns...)
}

func (sq *SnapshotQuery) prepareQuery(ctx context.Context) error {
	for _, inter := range sq.inters {
		if inter == nil {
			return fmt.Errorf("ent: uninitialized interceptor (forgotten import ent/runtime?)")
		}
		if trv, ok := inter.(Traverser); ok {
			if err := trv.Traverse(ctx, sq); err != nil {
				return err
			}
		}
	}
	for _, f := range sq.ctx.Fields {
		if !snapshot.ValidColumn(f) {
			return &ValidationError{Name: f, err: fmt.Errorf("ent: invalid field %q for query", f)}
		}
	}
	if sq.path != nil {
		prev, err := sq.path(ctx)
		if err != nil {
			return err
		}
		sq.sql = prev
	}
	return nil
}

func (sq *SnapshotQuery) sqlAll(ctx context.Context, hooks ...queryHook) ([]*Snapshot, error) {
	var (
		nodes       = []*Snapshot{}
		_spec       = sq.querySpec()
		loadedTypes = [1]bool{
			sq.withTopic != nil,
		}
	)
	_spec.ScanValues = func(columns []string) ([]any, error) {
		return (*Snapshot).scanValues(nil, columns)
	}
	_spec.Assign = func(columns []string, values []any) error {
		node := &Snapshot{config: sq.config}
		nodes = append(nodes, node)
		node.Edges.loadedTypes = loadedTypes
		return node.assignValues(columns, values)
	}
	if len(sq.modifiers) > 0 {
		_spec.Modifiers = sq.modifiers
	}
	for i := range hooks {
		hooks[i](ctx, _spec)
	}
	if err := sqlgraph.QueryNodes(ctx, sq.driver, _spec); err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nodes, nil
	}
	if query := sq.withTopic; query != nil {
		if err := sq.loadTopic(ctx, query, nodes, nil,
			func(n *Snapshot, e *Topic) { n.Edges.Topic = e }); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (sq *SnapshotQuery) loadTopic(ctx context.Context, query *TopicQuery, nodes []*Snapshot, init func(*Snapshot), assign func(*Snapshot, *Topic)) error {
	ids := make([]uuid.UUID, 0, len(nodes))
	nodeids := make(map[uuid.UUID][]*Snapshot)
	for i := range nodes {
		fk := nodes[i].TopicID
		if _, ok := nodeids[fk]; !ok {
			ids = append(ids, fk)
		}
		nodeids[fk] = append(nodeids[fk], nodes[i])
	}
	if len(ids) == 0 {
		return nil
	}
	query.Where(topic.IDIn(ids...))
	neighbors, err := query.All(ctx)
	if err != nil {
		return err
	}
	for _, n := range neighbors {
		nodes, ok := nodeids[n.ID]
		if !ok {
			return fmt.Errorf(`unexpected foreign-key "topicID" returned %v`, n.ID)
		}
		for i := range nodes {
			assign(nodes[i], n)
		}
	}
	return nil
}

func (sq *SnapshotQuery) sqlCount(ctx context.Context) (int, error) {
	_spec := sq.querySpec()
	if len(sq.modifiers) > 0 {
		_spec.Modifiers = sq.modifiers
	}
	_spec.Node.Columns = sq.ctx.Fields
	if len(sq.ctx.Fields) > 0 {
		_spec.Unique = sq.ctx.Unique != nil && *sq.ctx.Unique
	}
	return sqlgraph.CountNodes(ctx, sq.driver, _spec)
}

func (sq *SnapshotQuery) querySpec() *sqlgraph.QuerySpec {
	_spec := sqlgraph.NewQuerySpec(snapshot.Table, snapshot.Columns, sqlgraph.NewFieldSpec(snapshot.FieldID, field.TypeUUID))
	_spec.From = sq.sql
	if unique := sq.ctx.Unique; unique != nil {
		_spec.Unique = *unique
	} else if sq.path != nil {
		_spec.Unique = true
	}
	if fields := sq.ctx.Fields; len(fields) > 0 {
		_spec.Node.Columns = make([]string, 0, len(fields))
		_spec.Node.Columns = append(_spec.Node.Columns, snapshot.FieldID)
		for i := range fields {
			if fields[i] != snapshot.FieldID {
				_spec.Node.Columns = append(_spec.Node.Columns, fields[i])
			}
		}
		if sq.withTopic != nil {
			_spec.Node.AddColumnOnce(snapshot.FieldTopicID)
		}
	}
	if ps := sq.predicates; len(ps) > 0 {
		_spec.Predicate = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	if limit := sq.ctx.Limit; limit != nil {
		_spec.Limit = *limit
	}
	if offset := sq.ctx.Offset; offset != nil {
		_spec.Offset = *offset
	}
	if ps := sq.order; len(ps) > 0 {
		_spec.Order = func(selector *sql.Selector) {
			for i := range ps {
				ps[i](selector)
			}
		}
	}
	return _spec
}

func (sq *SnapshotQuery) sqlQuery(ctx context.Context) *sql.Selector {
	builder := sql.Dialect(sq.driver.Dialect())
	t1 := builder.Table(snapshot.Table)
	columns := sq.ctx.Fields
	if len(columns) == 0 {
		columns = snapshot.Columns
	}
	selector := builder.Select(t1.Columns(columns...)...).From(t1)
	if sq.sql != nil {
		selector = sq.sql
		selector.Select(selector.Columns(columns...)...)
	}
	if sq.ctx.Unique != nil && *sq.ctx.Unique {
		selector.Distinct()
	}
	for _, m := range sq.modifiers {
		m(selector)
	}
	for _, p := range sq.predicates {
		p(selector)
	}
	for _, p := range sq.order {
		p(selector)
	}
	if offset := sq.ctx.Offset; offset != nil {
		// limit is mandatory for offset clause. We start
		// with default value, and override it below if needed.
		selector.Offset(*offset).Limit(math.MaxInt32)
	}
	if limit := sq.ctx.Limit; limit != nil {
		selector.Limit(*limit)
	}
	return selector
}

// ForUpdate locks the selected rows against concurrent updates, and prevent them from being
// updated, deleted or "selected ... for update" by other sessions, until the transaction is
// either committed or rolled-back.
func (sq *SnapshotQuery) ForUpdate(opts ...sql.LockOption) *SnapshotQuery {
	if sq.driver.Dialect() == dialect.Postgres {
		sq.Unique(false)
	}
	sq.modifiers = append(sq.modifiers, func(s *sql.Selector) {
		s.ForUpdate(opts...)
	})
	return sq
}

// ForShare behaves similarly to ForUpdate, except that it acquires a shared mode lock
// on any rows that are read. Other sessions can read the rows, but cannot modify them
// until your transaction commits.
func (sq *SnapshotQuery) ForShare(opts ...sql.LockOption) *SnapshotQuery {
	if sq.driver.Dialect() == dialect.Postgres {
		sq.Unique(false)
	}
	sq.modifiers = append(sq.modifiers, func(s *sql.Selector) {
		s.ForShare(opts...)
	})
	return sq
}

// SnapshotGroupBy is the group-by builder for Snapshot entities.
type SnapshotGroupBy struct {
	selector
	build *SnapshotQuery
}

// Aggregate adds the given aggregation functions to the group-by query.
func (sgb *SnapshotGroupBy) Aggregate(fns ...AggregateFunc) *SnapshotGroupBy {
	sgb.fns = append(sgb.fns, fns...)
	return sgb
}

// Scan applies the selector query and scans the result into the given value.
func (sgb *SnapshotGroupBy) Scan(ctx context.Context, v any) error {
	ctx = setContextOp(ctx, sgb.build.ctx, ent.OpQueryGroupBy)
	if err := sgb.build.prepareQuery(ctx); err != nil {
		return err
	}
	return scanWithInterceptors[*SnapshotQuery, *SnapshotGroupBy](ctx, sgb.build, sgb, sgb.build.inters, v)
}

func (sgb *SnapshotGroupBy) sqlScan(ctx context.Context, root *SnapshotQuery, v any) error {
	selector := root.sqlQuery(ctx).Select()
	aggregation := make([]string, 0, len(sgb.fns))
	for _, fn := range sgb.fns {
		aggregation = append(aggregation, fn(selector))
	}
	if len(selector.SelectedColumns()) == 0 {
		columns := make([]string, 0, len(*sgb.flds)+len(sgb.fns))
		for _, f := range *sgb.flds {
			columns = append(columns, selector.C(f))
		}
		columns = append(columns, aggregation...)
		selector.Select(columns...)
	}
	selector.GroupBy(selector.Columns(*sgb.flds...)...)
	if err := selector.Err(); err != nil {
		return err
	}
	rows := &sql.Rows{}
	query, args := selector.Query()
	if err := sgb.build.driver.Query(ctx, query, args, rows); err != nil {
		return err
	}
	defer rows.Close()
	return sql.ScanSlice(rows, v)
}

// SnapshotSelect is the builder for selecting fields of Snapshot entities.
type SnapshotSelect struct {
	*SnapshotQuery
	selector
}

// Aggregate adds the given aggregation functions to the selector query.
func (ss *SnapshotSelect) Aggregate(fns ...AggregateFunc) *SnapshotSelect {
	ss.fns = append(ss.fns, fns...)
	return ss
}

// Scan applies the selector query and scans the result into the given value.
func (ss *SnapshotSelect) Scan(ctx context.Context, v any) error {
	ctx = setContextOp(ctx, ss.ctx, ent.OpQuerySelect)
	if err := ss.prepareQuery(ctx); err != nil {
		return err
	}
	return scanWithInterceptors[*SnapshotQuery, *SnapshotSelect](ctx, ss.SnapshotQuery, ss, ss.inters, v)
}

func (ss *SnapshotSelect) sqlScan(ctx context.Context, root *SnapshotQuery, v any) error {
	selector := root.sqlQuery(ctx)
	aggregation := make([]string, 0, len(ss.fns))
	for _, fn := range ss.fns {
		aggregation = append(aggregation, fn(selector))
	}
	switch n := len(*ss.selector.flds); {
	case n == 0 && len(aggregation) > 0:
		selector.Select(aggregation...)
	case n != 0 && len(aggregation) > 0:
		selector.AppendSelect(aggregation...)
	}
	rows := &sql.Rows{}
	query, args := selector.Query()
	if err := ss.driver.Query(ctx, query, args, rows); err != nil {
		return err
	}
	defer rows.Close()
	return sql.ScanSlice(rows, v)
}
