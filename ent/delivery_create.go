// Code generated by ent, DO NOT EDIT.

package ent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/subscription"
)

// DeliveryCreate is the builder for creating a Delivery entity.
type DeliveryCreate struct {
	config
	mutation *DeliveryMutation
	hooks    []Hook
	conflict []sql.ConflictOption
}

// SetMessageID sets the "messageID" field.
func (dc *DeliveryCreate) SetMessageID(u uuid.UUID) *DeliveryCreate {
	dc.mutation.SetMessageID(u)
	return dc
}

// SetSubscriptionID sets the "subscriptionID" field.
func (dc *DeliveryCreate) SetSubscriptionID(u uuid.UUID) *DeliveryCreate {
	dc.mutation.SetSubscriptionID(u)
	return dc
}

// SetPublishedAt sets the "publishedAt" field.
func (dc *DeliveryCreate) SetPublishedAt(t time.Time) *DeliveryCreate {
	dc.mutation.SetPublishedAt(t)
	return dc
}

// SetNillablePublishedAt sets the "publishedAt" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillablePublishedAt(t *time.Time) *DeliveryCreate {
	if t != nil {
		dc.SetPublishedAt(*t)
	}
	return dc
}

// SetAttemptAt sets the "attemptAt" field.
func (dc *DeliveryCreate) SetAttemptAt(t time.Time) *DeliveryCreate {
	dc.mutation.SetAttemptAt(t)
	return dc
}

// SetNillableAttemptAt sets the "attemptAt" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableAttemptAt(t *time.Time) *DeliveryCreate {
	if t != nil {
		dc.SetAttemptAt(*t)
	}
	return dc
}

// SetLastAttemptedAt sets the "lastAttemptedAt" field.
func (dc *DeliveryCreate) SetLastAttemptedAt(t time.Time) *DeliveryCreate {
	dc.mutation.SetLastAttemptedAt(t)
	return dc
}

// SetNillableLastAttemptedAt sets the "lastAttemptedAt" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableLastAttemptedAt(t *time.Time) *DeliveryCreate {
	if t != nil {
		dc.SetLastAttemptedAt(*t)
	}
	return dc
}

// SetAttempts sets the "attempts" field.
func (dc *DeliveryCreate) SetAttempts(i int) *DeliveryCreate {
	dc.mutation.SetAttempts(i)
	return dc
}

// SetNillableAttempts sets the "attempts" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableAttempts(i *int) *DeliveryCreate {
	if i != nil {
		dc.SetAttempts(*i)
	}
	return dc
}

// SetCompletedAt sets the "completedAt" field.
func (dc *DeliveryCreate) SetCompletedAt(t time.Time) *DeliveryCreate {
	dc.mutation.SetCompletedAt(t)
	return dc
}

// SetNillableCompletedAt sets the "completedAt" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableCompletedAt(t *time.Time) *DeliveryCreate {
	if t != nil {
		dc.SetCompletedAt(*t)
	}
	return dc
}

// SetExpiresAt sets the "expiresAt" field.
func (dc *DeliveryCreate) SetExpiresAt(t time.Time) *DeliveryCreate {
	dc.mutation.SetExpiresAt(t)
	return dc
}

// SetNotBeforeID sets the "notBeforeID" field.
func (dc *DeliveryCreate) SetNotBeforeID(u uuid.UUID) *DeliveryCreate {
	dc.mutation.SetNotBeforeID(u)
	return dc
}

// SetNillableNotBeforeID sets the "notBeforeID" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableNotBeforeID(u *uuid.UUID) *DeliveryCreate {
	if u != nil {
		dc.SetNotBeforeID(*u)
	}
	return dc
}

// SetID sets the "id" field.
func (dc *DeliveryCreate) SetID(u uuid.UUID) *DeliveryCreate {
	dc.mutation.SetID(u)
	return dc
}

// SetNillableID sets the "id" field if the given value is not nil.
func (dc *DeliveryCreate) SetNillableID(u *uuid.UUID) *DeliveryCreate {
	if u != nil {
		dc.SetID(*u)
	}
	return dc
}

// SetMessage sets the "message" edge to the Message entity.
func (dc *DeliveryCreate) SetMessage(m *Message) *DeliveryCreate {
	return dc.SetMessageID(m.ID)
}

// SetSubscription sets the "subscription" edge to the Subscription entity.
func (dc *DeliveryCreate) SetSubscription(s *Subscription) *DeliveryCreate {
	return dc.SetSubscriptionID(s.ID)
}

// SetNotBefore sets the "notBefore" edge to the Delivery entity.
func (dc *DeliveryCreate) SetNotBefore(d *Delivery) *DeliveryCreate {
	return dc.SetNotBeforeID(d.ID)
}

// AddNextReadyIDs adds the "nextReady" edge to the Delivery entity by IDs.
func (dc *DeliveryCreate) AddNextReadyIDs(ids ...uuid.UUID) *DeliveryCreate {
	dc.mutation.AddNextReadyIDs(ids...)
	return dc
}

// AddNextReady adds the "nextReady" edges to the Delivery entity.
func (dc *DeliveryCreate) AddNextReady(d ...*Delivery) *DeliveryCreate {
	ids := make([]uuid.UUID, len(d))
	for i := range d {
		ids[i] = d[i].ID
	}
	return dc.AddNextReadyIDs(ids...)
}

// Mutation returns the DeliveryMutation object of the builder.
func (dc *DeliveryCreate) Mutation() *DeliveryMutation {
	return dc.mutation
}

// Save creates the Delivery in the database.
func (dc *DeliveryCreate) Save(ctx context.Context) (*Delivery, error) {
	dc.defaults()
	return withHooks(ctx, dc.sqlSave, dc.mutation, dc.hooks)
}

// SaveX calls Save and panics if Save returns an error.
func (dc *DeliveryCreate) SaveX(ctx context.Context) *Delivery {
	v, err := dc.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (dc *DeliveryCreate) Exec(ctx context.Context) error {
	_, err := dc.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (dc *DeliveryCreate) ExecX(ctx context.Context) {
	if err := dc.Exec(ctx); err != nil {
		panic(err)
	}
}

// defaults sets the default values of the builder before save.
func (dc *DeliveryCreate) defaults() {
	if _, ok := dc.mutation.PublishedAt(); !ok {
		v := delivery.DefaultPublishedAt()
		dc.mutation.SetPublishedAt(v)
	}
	if _, ok := dc.mutation.AttemptAt(); !ok {
		v := delivery.DefaultAttemptAt()
		dc.mutation.SetAttemptAt(v)
	}
	if _, ok := dc.mutation.Attempts(); !ok {
		v := delivery.DefaultAttempts
		dc.mutation.SetAttempts(v)
	}
	if _, ok := dc.mutation.ID(); !ok {
		v := delivery.DefaultID()
		dc.mutation.SetID(v)
	}
}

// check runs all checks and user-defined validators on the builder.
func (dc *DeliveryCreate) check() error {
	if _, ok := dc.mutation.MessageID(); !ok {
		return &ValidationError{Name: "messageID", err: errors.New(`ent: missing required field "Delivery.messageID"`)}
	}
	if _, ok := dc.mutation.SubscriptionID(); !ok {
		return &ValidationError{Name: "subscriptionID", err: errors.New(`ent: missing required field "Delivery.subscriptionID"`)}
	}
	if _, ok := dc.mutation.PublishedAt(); !ok {
		return &ValidationError{Name: "publishedAt", err: errors.New(`ent: missing required field "Delivery.publishedAt"`)}
	}
	if _, ok := dc.mutation.AttemptAt(); !ok {
		return &ValidationError{Name: "attemptAt", err: errors.New(`ent: missing required field "Delivery.attemptAt"`)}
	}
	if _, ok := dc.mutation.Attempts(); !ok {
		return &ValidationError{Name: "attempts", err: errors.New(`ent: missing required field "Delivery.attempts"`)}
	}
	if _, ok := dc.mutation.ExpiresAt(); !ok {
		return &ValidationError{Name: "expiresAt", err: errors.New(`ent: missing required field "Delivery.expiresAt"`)}
	}
	if len(dc.mutation.MessageIDs()) == 0 {
		return &ValidationError{Name: "message", err: errors.New(`ent: missing required edge "Delivery.message"`)}
	}
	if len(dc.mutation.SubscriptionIDs()) == 0 {
		return &ValidationError{Name: "subscription", err: errors.New(`ent: missing required edge "Delivery.subscription"`)}
	}
	return nil
}

func (dc *DeliveryCreate) sqlSave(ctx context.Context) (*Delivery, error) {
	if err := dc.check(); err != nil {
		return nil, err
	}
	_node, _spec := dc.createSpec()
	if err := sqlgraph.CreateNode(ctx, dc.driver, _spec); err != nil {
		if sqlgraph.IsConstraintError(err) {
			err = &ConstraintError{msg: err.Error(), wrap: err}
		}
		return nil, err
	}
	if _spec.ID.Value != nil {
		if id, ok := _spec.ID.Value.(*uuid.UUID); ok {
			_node.ID = *id
		} else if err := _node.ID.Scan(_spec.ID.Value); err != nil {
			return nil, err
		}
	}
	dc.mutation.id = &_node.ID
	dc.mutation.done = true
	return _node, nil
}

func (dc *DeliveryCreate) createSpec() (*Delivery, *sqlgraph.CreateSpec) {
	var (
		_node = &Delivery{config: dc.config}
		_spec = sqlgraph.NewCreateSpec(delivery.Table, sqlgraph.NewFieldSpec(delivery.FieldID, field.TypeUUID))
	)
	_spec.OnConflict = dc.conflict
	if id, ok := dc.mutation.ID(); ok {
		_node.ID = id
		_spec.ID.Value = &id
	}
	if value, ok := dc.mutation.PublishedAt(); ok {
		_spec.SetField(delivery.FieldPublishedAt, field.TypeTime, value)
		_node.PublishedAt = value
	}
	if value, ok := dc.mutation.AttemptAt(); ok {
		_spec.SetField(delivery.FieldAttemptAt, field.TypeTime, value)
		_node.AttemptAt = value
	}
	if value, ok := dc.mutation.LastAttemptedAt(); ok {
		_spec.SetField(delivery.FieldLastAttemptedAt, field.TypeTime, value)
		_node.LastAttemptedAt = &value
	}
	if value, ok := dc.mutation.Attempts(); ok {
		_spec.SetField(delivery.FieldAttempts, field.TypeInt, value)
		_node.Attempts = value
	}
	if value, ok := dc.mutation.CompletedAt(); ok {
		_spec.SetField(delivery.FieldCompletedAt, field.TypeTime, value)
		_node.CompletedAt = &value
	}
	if value, ok := dc.mutation.ExpiresAt(); ok {
		_spec.SetField(delivery.FieldExpiresAt, field.TypeTime, value)
		_node.ExpiresAt = value
	}
	if nodes := dc.mutation.MessageIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   delivery.MessageTable,
			Columns: []string{delivery.MessageColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(message.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_node.MessageID = nodes[0]
		_spec.Edges = append(_spec.Edges, edge)
	}
	if nodes := dc.mutation.SubscriptionIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: false,
			Table:   delivery.SubscriptionTable,
			Columns: []string{delivery.SubscriptionColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(subscription.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_node.SubscriptionID = nodes[0]
		_spec.Edges = append(_spec.Edges, edge)
	}
	if nodes := dc.mutation.NotBeforeIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.M2O,
			Inverse: true,
			Table:   delivery.NotBeforeTable,
			Columns: []string{delivery.NotBeforeColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(delivery.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_node.NotBeforeID = nodes[0]
		_spec.Edges = append(_spec.Edges, edge)
	}
	if nodes := dc.mutation.NextReadyIDs(); len(nodes) > 0 {
		edge := &sqlgraph.EdgeSpec{
			Rel:     sqlgraph.O2M,
			Inverse: false,
			Table:   delivery.NextReadyTable,
			Columns: []string{delivery.NextReadyColumn},
			Bidi:    false,
			Target: &sqlgraph.EdgeTarget{
				IDSpec: sqlgraph.NewFieldSpec(delivery.FieldID, field.TypeUUID),
			},
		}
		for _, k := range nodes {
			edge.Target.Nodes = append(edge.Target.Nodes, k)
		}
		_spec.Edges = append(_spec.Edges, edge)
	}
	return _node, _spec
}

// OnConflict allows configuring the `ON CONFLICT` / `ON DUPLICATE KEY` clause
// of the `INSERT` statement. For example:
//
//	client.Delivery.Create().
//		SetMessageID(v).
//		OnConflict(
//			// Update the row with the new values
//			// the was proposed for insertion.
//			sql.ResolveWithNewValues(),
//		).
//		// Override some of the fields with custom
//		// update values.
//		Update(func(u *ent.DeliveryUpsert) {
//			SetMessageID(v+v).
//		}).
//		Exec(ctx)
func (dc *DeliveryCreate) OnConflict(opts ...sql.ConflictOption) *DeliveryUpsertOne {
	dc.conflict = opts
	return &DeliveryUpsertOne{
		create: dc,
	}
}

// OnConflictColumns calls `OnConflict` and configures the columns
// as conflict target. Using this option is equivalent to using:
//
//	client.Delivery.Create().
//		OnConflict(sql.ConflictColumns(columns...)).
//		Exec(ctx)
func (dc *DeliveryCreate) OnConflictColumns(columns ...string) *DeliveryUpsertOne {
	dc.conflict = append(dc.conflict, sql.ConflictColumns(columns...))
	return &DeliveryUpsertOne{
		create: dc,
	}
}

type (
	// DeliveryUpsertOne is the builder for "upsert"-ing
	//  one Delivery node.
	DeliveryUpsertOne struct {
		create *DeliveryCreate
	}

	// DeliveryUpsert is the "OnConflict" setter.
	DeliveryUpsert struct {
		*sql.UpdateSet
	}
)

// SetMessageID sets the "messageID" field.
func (u *DeliveryUpsert) SetMessageID(v uuid.UUID) *DeliveryUpsert {
	u.Set(delivery.FieldMessageID, v)
	return u
}

// UpdateMessageID sets the "messageID" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateMessageID() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldMessageID)
	return u
}

// SetSubscriptionID sets the "subscriptionID" field.
func (u *DeliveryUpsert) SetSubscriptionID(v uuid.UUID) *DeliveryUpsert {
	u.Set(delivery.FieldSubscriptionID, v)
	return u
}

// UpdateSubscriptionID sets the "subscriptionID" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateSubscriptionID() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldSubscriptionID)
	return u
}

// SetPublishedAt sets the "publishedAt" field.
func (u *DeliveryUpsert) SetPublishedAt(v time.Time) *DeliveryUpsert {
	u.Set(delivery.FieldPublishedAt, v)
	return u
}

// UpdatePublishedAt sets the "publishedAt" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdatePublishedAt() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldPublishedAt)
	return u
}

// SetAttemptAt sets the "attemptAt" field.
func (u *DeliveryUpsert) SetAttemptAt(v time.Time) *DeliveryUpsert {
	u.Set(delivery.FieldAttemptAt, v)
	return u
}

// UpdateAttemptAt sets the "attemptAt" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateAttemptAt() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldAttemptAt)
	return u
}

// SetLastAttemptedAt sets the "lastAttemptedAt" field.
func (u *DeliveryUpsert) SetLastAttemptedAt(v time.Time) *DeliveryUpsert {
	u.Set(delivery.FieldLastAttemptedAt, v)
	return u
}

// UpdateLastAttemptedAt sets the "lastAttemptedAt" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateLastAttemptedAt() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldLastAttemptedAt)
	return u
}

// ClearLastAttemptedAt clears the value of the "lastAttemptedAt" field.
func (u *DeliveryUpsert) ClearLastAttemptedAt() *DeliveryUpsert {
	u.SetNull(delivery.FieldLastAttemptedAt)
	return u
}

// SetAttempts sets the "attempts" field.
func (u *DeliveryUpsert) SetAttempts(v int) *DeliveryUpsert {
	u.Set(delivery.FieldAttempts, v)
	return u
}

// UpdateAttempts sets the "attempts" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateAttempts() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldAttempts)
	return u
}

// AddAttempts adds v to the "attempts" field.
func (u *DeliveryUpsert) AddAttempts(v int) *DeliveryUpsert {
	u.Add(delivery.FieldAttempts, v)
	return u
}

// SetCompletedAt sets the "completedAt" field.
func (u *DeliveryUpsert) SetCompletedAt(v time.Time) *DeliveryUpsert {
	u.Set(delivery.FieldCompletedAt, v)
	return u
}

// UpdateCompletedAt sets the "completedAt" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateCompletedAt() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldCompletedAt)
	return u
}

// ClearCompletedAt clears the value of the "completedAt" field.
func (u *DeliveryUpsert) ClearCompletedAt() *DeliveryUpsert {
	u.SetNull(delivery.FieldCompletedAt)
	return u
}

// SetExpiresAt sets the "expiresAt" field.
func (u *DeliveryUpsert) SetExpiresAt(v time.Time) *DeliveryUpsert {
	u.Set(delivery.FieldExpiresAt, v)
	return u
}

// UpdateExpiresAt sets the "expiresAt" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateExpiresAt() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldExpiresAt)
	return u
}

// SetNotBeforeID sets the "notBeforeID" field.
func (u *DeliveryUpsert) SetNotBeforeID(v uuid.UUID) *DeliveryUpsert {
	u.Set(delivery.FieldNotBeforeID, v)
	return u
}

// UpdateNotBeforeID sets the "notBeforeID" field to the value that was provided on create.
func (u *DeliveryUpsert) UpdateNotBeforeID() *DeliveryUpsert {
	u.SetExcluded(delivery.FieldNotBeforeID)
	return u
}

// ClearNotBeforeID clears the value of the "notBeforeID" field.
func (u *DeliveryUpsert) ClearNotBeforeID() *DeliveryUpsert {
	u.SetNull(delivery.FieldNotBeforeID)
	return u
}

// UpdateNewValues updates the mutable fields using the new values that were set on create except the ID field.
// Using this option is equivalent to using:
//
//	client.Delivery.Create().
//		OnConflict(
//			sql.ResolveWithNewValues(),
//			sql.ResolveWith(func(u *sql.UpdateSet) {
//				u.SetIgnore(delivery.FieldID)
//			}),
//		).
//		Exec(ctx)
func (u *DeliveryUpsertOne) UpdateNewValues() *DeliveryUpsertOne {
	u.create.conflict = append(u.create.conflict, sql.ResolveWithNewValues())
	u.create.conflict = append(u.create.conflict, sql.ResolveWith(func(s *sql.UpdateSet) {
		if _, exists := u.create.mutation.ID(); exists {
			s.SetIgnore(delivery.FieldID)
		}
	}))
	return u
}

// Ignore sets each column to itself in case of conflict.
// Using this option is equivalent to using:
//
//	client.Delivery.Create().
//	    OnConflict(sql.ResolveWithIgnore()).
//	    Exec(ctx)
func (u *DeliveryUpsertOne) Ignore() *DeliveryUpsertOne {
	u.create.conflict = append(u.create.conflict, sql.ResolveWithIgnore())
	return u
}

// DoNothing configures the conflict_action to `DO NOTHING`.
// Supported only by SQLite and PostgreSQL.
func (u *DeliveryUpsertOne) DoNothing() *DeliveryUpsertOne {
	u.create.conflict = append(u.create.conflict, sql.DoNothing())
	return u
}

// Update allows overriding fields `UPDATE` values. See the DeliveryCreate.OnConflict
// documentation for more info.
func (u *DeliveryUpsertOne) Update(set func(*DeliveryUpsert)) *DeliveryUpsertOne {
	u.create.conflict = append(u.create.conflict, sql.ResolveWith(func(update *sql.UpdateSet) {
		set(&DeliveryUpsert{UpdateSet: update})
	}))
	return u
}

// SetMessageID sets the "messageID" field.
func (u *DeliveryUpsertOne) SetMessageID(v uuid.UUID) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetMessageID(v)
	})
}

// UpdateMessageID sets the "messageID" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateMessageID() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateMessageID()
	})
}

// SetSubscriptionID sets the "subscriptionID" field.
func (u *DeliveryUpsertOne) SetSubscriptionID(v uuid.UUID) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetSubscriptionID(v)
	})
}

// UpdateSubscriptionID sets the "subscriptionID" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateSubscriptionID() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateSubscriptionID()
	})
}

// SetPublishedAt sets the "publishedAt" field.
func (u *DeliveryUpsertOne) SetPublishedAt(v time.Time) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetPublishedAt(v)
	})
}

// UpdatePublishedAt sets the "publishedAt" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdatePublishedAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdatePublishedAt()
	})
}

// SetAttemptAt sets the "attemptAt" field.
func (u *DeliveryUpsertOne) SetAttemptAt(v time.Time) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetAttemptAt(v)
	})
}

// UpdateAttemptAt sets the "attemptAt" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateAttemptAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateAttemptAt()
	})
}

// SetLastAttemptedAt sets the "lastAttemptedAt" field.
func (u *DeliveryUpsertOne) SetLastAttemptedAt(v time.Time) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetLastAttemptedAt(v)
	})
}

// UpdateLastAttemptedAt sets the "lastAttemptedAt" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateLastAttemptedAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateLastAttemptedAt()
	})
}

// ClearLastAttemptedAt clears the value of the "lastAttemptedAt" field.
func (u *DeliveryUpsertOne) ClearLastAttemptedAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearLastAttemptedAt()
	})
}

// SetAttempts sets the "attempts" field.
func (u *DeliveryUpsertOne) SetAttempts(v int) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetAttempts(v)
	})
}

// AddAttempts adds v to the "attempts" field.
func (u *DeliveryUpsertOne) AddAttempts(v int) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.AddAttempts(v)
	})
}

// UpdateAttempts sets the "attempts" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateAttempts() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateAttempts()
	})
}

// SetCompletedAt sets the "completedAt" field.
func (u *DeliveryUpsertOne) SetCompletedAt(v time.Time) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetCompletedAt(v)
	})
}

// UpdateCompletedAt sets the "completedAt" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateCompletedAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateCompletedAt()
	})
}

// ClearCompletedAt clears the value of the "completedAt" field.
func (u *DeliveryUpsertOne) ClearCompletedAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearCompletedAt()
	})
}

// SetExpiresAt sets the "expiresAt" field.
func (u *DeliveryUpsertOne) SetExpiresAt(v time.Time) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetExpiresAt(v)
	})
}

// UpdateExpiresAt sets the "expiresAt" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateExpiresAt() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateExpiresAt()
	})
}

// SetNotBeforeID sets the "notBeforeID" field.
func (u *DeliveryUpsertOne) SetNotBeforeID(v uuid.UUID) *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetNotBeforeID(v)
	})
}

// UpdateNotBeforeID sets the "notBeforeID" field to the value that was provided on create.
func (u *DeliveryUpsertOne) UpdateNotBeforeID() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateNotBeforeID()
	})
}

// ClearNotBeforeID clears the value of the "notBeforeID" field.
func (u *DeliveryUpsertOne) ClearNotBeforeID() *DeliveryUpsertOne {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearNotBeforeID()
	})
}

// Exec executes the query.
func (u *DeliveryUpsertOne) Exec(ctx context.Context) error {
	if len(u.create.conflict) == 0 {
		return errors.New("ent: missing options for DeliveryCreate.OnConflict")
	}
	return u.create.Exec(ctx)
}

// ExecX is like Exec, but panics if an error occurs.
func (u *DeliveryUpsertOne) ExecX(ctx context.Context) {
	if err := u.create.Exec(ctx); err != nil {
		panic(err)
	}
}

// Exec executes the UPSERT query and returns the inserted/updated ID.
func (u *DeliveryUpsertOne) ID(ctx context.Context) (id uuid.UUID, err error) {
	if u.create.driver.Dialect() == dialect.MySQL {
		// In case of "ON CONFLICT", there is no way to get back non-numeric ID
		// fields from the database since MySQL does not support the RETURNING clause.
		return id, errors.New("ent: DeliveryUpsertOne.ID is not supported by MySQL driver. Use DeliveryUpsertOne.Exec instead")
	}
	node, err := u.create.Save(ctx)
	if err != nil {
		return id, err
	}
	return node.ID, nil
}

// IDX is like ID, but panics if an error occurs.
func (u *DeliveryUpsertOne) IDX(ctx context.Context) uuid.UUID {
	id, err := u.ID(ctx)
	if err != nil {
		panic(err)
	}
	return id
}

// DeliveryCreateBulk is the builder for creating many Delivery entities in bulk.
type DeliveryCreateBulk struct {
	config
	err      error
	builders []*DeliveryCreate
	conflict []sql.ConflictOption
}

// Save creates the Delivery entities in the database.
func (dcb *DeliveryCreateBulk) Save(ctx context.Context) ([]*Delivery, error) {
	if dcb.err != nil {
		return nil, dcb.err
	}
	specs := make([]*sqlgraph.CreateSpec, len(dcb.builders))
	nodes := make([]*Delivery, len(dcb.builders))
	mutators := make([]Mutator, len(dcb.builders))
	for i := range dcb.builders {
		func(i int, root context.Context) {
			builder := dcb.builders[i]
			builder.defaults()
			var mut Mutator = MutateFunc(func(ctx context.Context, m Mutation) (Value, error) {
				mutation, ok := m.(*DeliveryMutation)
				if !ok {
					return nil, fmt.Errorf("unexpected mutation type %T", m)
				}
				if err := builder.check(); err != nil {
					return nil, err
				}
				builder.mutation = mutation
				var err error
				nodes[i], specs[i] = builder.createSpec()
				if i < len(mutators)-1 {
					_, err = mutators[i+1].Mutate(root, dcb.builders[i+1].mutation)
				} else {
					spec := &sqlgraph.BatchCreateSpec{Nodes: specs}
					spec.OnConflict = dcb.conflict
					// Invoke the actual operation on the latest mutation in the chain.
					if err = sqlgraph.BatchCreate(ctx, dcb.driver, spec); err != nil {
						if sqlgraph.IsConstraintError(err) {
							err = &ConstraintError{msg: err.Error(), wrap: err}
						}
					}
				}
				if err != nil {
					return nil, err
				}
				mutation.id = &nodes[i].ID
				mutation.done = true
				return nodes[i], nil
			})
			for i := len(builder.hooks) - 1; i >= 0; i-- {
				mut = builder.hooks[i](mut)
			}
			mutators[i] = mut
		}(i, ctx)
	}
	if len(mutators) > 0 {
		if _, err := mutators[0].Mutate(ctx, dcb.builders[0].mutation); err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

// SaveX is like Save, but panics if an error occurs.
func (dcb *DeliveryCreateBulk) SaveX(ctx context.Context) []*Delivery {
	v, err := dcb.Save(ctx)
	if err != nil {
		panic(err)
	}
	return v
}

// Exec executes the query.
func (dcb *DeliveryCreateBulk) Exec(ctx context.Context) error {
	_, err := dcb.Save(ctx)
	return err
}

// ExecX is like Exec, but panics if an error occurs.
func (dcb *DeliveryCreateBulk) ExecX(ctx context.Context) {
	if err := dcb.Exec(ctx); err != nil {
		panic(err)
	}
}

// OnConflict allows configuring the `ON CONFLICT` / `ON DUPLICATE KEY` clause
// of the `INSERT` statement. For example:
//
//	client.Delivery.CreateBulk(builders...).
//		OnConflict(
//			// Update the row with the new values
//			// the was proposed for insertion.
//			sql.ResolveWithNewValues(),
//		).
//		// Override some of the fields with custom
//		// update values.
//		Update(func(u *ent.DeliveryUpsert) {
//			SetMessageID(v+v).
//		}).
//		Exec(ctx)
func (dcb *DeliveryCreateBulk) OnConflict(opts ...sql.ConflictOption) *DeliveryUpsertBulk {
	dcb.conflict = opts
	return &DeliveryUpsertBulk{
		create: dcb,
	}
}

// OnConflictColumns calls `OnConflict` and configures the columns
// as conflict target. Using this option is equivalent to using:
//
//	client.Delivery.Create().
//		OnConflict(sql.ConflictColumns(columns...)).
//		Exec(ctx)
func (dcb *DeliveryCreateBulk) OnConflictColumns(columns ...string) *DeliveryUpsertBulk {
	dcb.conflict = append(dcb.conflict, sql.ConflictColumns(columns...))
	return &DeliveryUpsertBulk{
		create: dcb,
	}
}

// DeliveryUpsertBulk is the builder for "upsert"-ing
// a bulk of Delivery nodes.
type DeliveryUpsertBulk struct {
	create *DeliveryCreateBulk
}

// UpdateNewValues updates the mutable fields using the new values that
// were set on create. Using this option is equivalent to using:
//
//	client.Delivery.Create().
//		OnConflict(
//			sql.ResolveWithNewValues(),
//			sql.ResolveWith(func(u *sql.UpdateSet) {
//				u.SetIgnore(delivery.FieldID)
//			}),
//		).
//		Exec(ctx)
func (u *DeliveryUpsertBulk) UpdateNewValues() *DeliveryUpsertBulk {
	u.create.conflict = append(u.create.conflict, sql.ResolveWithNewValues())
	u.create.conflict = append(u.create.conflict, sql.ResolveWith(func(s *sql.UpdateSet) {
		for _, b := range u.create.builders {
			if _, exists := b.mutation.ID(); exists {
				s.SetIgnore(delivery.FieldID)
			}
		}
	}))
	return u
}

// Ignore sets each column to itself in case of conflict.
// Using this option is equivalent to using:
//
//	client.Delivery.Create().
//		OnConflict(sql.ResolveWithIgnore()).
//		Exec(ctx)
func (u *DeliveryUpsertBulk) Ignore() *DeliveryUpsertBulk {
	u.create.conflict = append(u.create.conflict, sql.ResolveWithIgnore())
	return u
}

// DoNothing configures the conflict_action to `DO NOTHING`.
// Supported only by SQLite and PostgreSQL.
func (u *DeliveryUpsertBulk) DoNothing() *DeliveryUpsertBulk {
	u.create.conflict = append(u.create.conflict, sql.DoNothing())
	return u
}

// Update allows overriding fields `UPDATE` values. See the DeliveryCreateBulk.OnConflict
// documentation for more info.
func (u *DeliveryUpsertBulk) Update(set func(*DeliveryUpsert)) *DeliveryUpsertBulk {
	u.create.conflict = append(u.create.conflict, sql.ResolveWith(func(update *sql.UpdateSet) {
		set(&DeliveryUpsert{UpdateSet: update})
	}))
	return u
}

// SetMessageID sets the "messageID" field.
func (u *DeliveryUpsertBulk) SetMessageID(v uuid.UUID) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetMessageID(v)
	})
}

// UpdateMessageID sets the "messageID" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateMessageID() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateMessageID()
	})
}

// SetSubscriptionID sets the "subscriptionID" field.
func (u *DeliveryUpsertBulk) SetSubscriptionID(v uuid.UUID) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetSubscriptionID(v)
	})
}

// UpdateSubscriptionID sets the "subscriptionID" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateSubscriptionID() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateSubscriptionID()
	})
}

// SetPublishedAt sets the "publishedAt" field.
func (u *DeliveryUpsertBulk) SetPublishedAt(v time.Time) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetPublishedAt(v)
	})
}

// UpdatePublishedAt sets the "publishedAt" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdatePublishedAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdatePublishedAt()
	})
}

// SetAttemptAt sets the "attemptAt" field.
func (u *DeliveryUpsertBulk) SetAttemptAt(v time.Time) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetAttemptAt(v)
	})
}

// UpdateAttemptAt sets the "attemptAt" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateAttemptAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateAttemptAt()
	})
}

// SetLastAttemptedAt sets the "lastAttemptedAt" field.
func (u *DeliveryUpsertBulk) SetLastAttemptedAt(v time.Time) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetLastAttemptedAt(v)
	})
}

// UpdateLastAttemptedAt sets the "lastAttemptedAt" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateLastAttemptedAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateLastAttemptedAt()
	})
}

// ClearLastAttemptedAt clears the value of the "lastAttemptedAt" field.
func (u *DeliveryUpsertBulk) ClearLastAttemptedAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearLastAttemptedAt()
	})
}

// SetAttempts sets the "attempts" field.
func (u *DeliveryUpsertBulk) SetAttempts(v int) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetAttempts(v)
	})
}

// AddAttempts adds v to the "attempts" field.
func (u *DeliveryUpsertBulk) AddAttempts(v int) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.AddAttempts(v)
	})
}

// UpdateAttempts sets the "attempts" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateAttempts() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateAttempts()
	})
}

// SetCompletedAt sets the "completedAt" field.
func (u *DeliveryUpsertBulk) SetCompletedAt(v time.Time) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetCompletedAt(v)
	})
}

// UpdateCompletedAt sets the "completedAt" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateCompletedAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateCompletedAt()
	})
}

// ClearCompletedAt clears the value of the "completedAt" field.
func (u *DeliveryUpsertBulk) ClearCompletedAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearCompletedAt()
	})
}

// SetExpiresAt sets the "expiresAt" field.
func (u *DeliveryUpsertBulk) SetExpiresAt(v time.Time) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetExpiresAt(v)
	})
}

// UpdateExpiresAt sets the "expiresAt" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateExpiresAt() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateExpiresAt()
	})
}

// SetNotBeforeID sets the "notBeforeID" field.
func (u *DeliveryUpsertBulk) SetNotBeforeID(v uuid.UUID) *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.SetNotBeforeID(v)
	})
}

// UpdateNotBeforeID sets the "notBeforeID" field to the value that was provided on create.
func (u *DeliveryUpsertBulk) UpdateNotBeforeID() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.UpdateNotBeforeID()
	})
}

// ClearNotBeforeID clears the value of the "notBeforeID" field.
func (u *DeliveryUpsertBulk) ClearNotBeforeID() *DeliveryUpsertBulk {
	return u.Update(func(s *DeliveryUpsert) {
		s.ClearNotBeforeID()
	})
}

// Exec executes the query.
func (u *DeliveryUpsertBulk) Exec(ctx context.Context) error {
	if u.create.err != nil {
		return u.create.err
	}
	for i, b := range u.create.builders {
		if len(b.conflict) != 0 {
			return fmt.Errorf("ent: OnConflict was set for builder %d. Set it on the DeliveryCreateBulk instead", i)
		}
	}
	if len(u.create.conflict) == 0 {
		return errors.New("ent: missing options for DeliveryCreateBulk.OnConflict")
	}
	return u.create.Exec(ctx)
}

// ExecX is like Exec, but panics if an error occurs.
func (u *DeliveryUpsertBulk) ExecX(ctx context.Context) {
	if err := u.create.Exec(ctx); err != nil {
		panic(err)
	}
}