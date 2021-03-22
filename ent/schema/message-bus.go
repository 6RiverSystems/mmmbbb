package schema

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"

	"go.6river.tech/gosix/ent/customtypes"
)

var textTypes = map[string]string{
	dialect.Postgres: "text",
	dialect.SQLite:   "text",
}
var timestampTypes = map[string]string{
	dialect.Postgres: "timestamptz",
}
var intervalTypes = map[string]string{
	dialect.Postgres: "interval",
}
var jsonTypes = map[string]string{
	dialect.Postgres: "jsonb",
	dialect.SQLite:   "json", // really text
}

// Topic represents a destination to which messages can be published. Publishing
// a message to a topic sends it to all its subscriptions.
type Topic struct {
	ent.Schema
}

func (Topic) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "topics"},
		edge.Annotation{
			// don't serialize edges to JSON
			StructTag: `json:"-"`,
		},
	}
}

// Subscription represents a source from which messages can be received.
// Messages received on a subscription are those published to its linked topic.
type Subscription struct {
	ent.Schema
}

func (Subscription) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscriptions"},
		edge.Annotation{
			// don't serialize edges to JSON
			StructTag: `json:"-"`,
		},
	}
}

// Message represents a message sent on the bus.
type Message struct {
	ent.Schema
}

func (Message) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "messages"},
		edge.Annotation{
			// don't serialize edges to JSON
			StructTag: `json:"-"`,
		},
	}
}

// Delivery represents the status of delivering a message to a subscription.
type Delivery struct {
	ent.Schema
}

func (Delivery) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "deliveries"},
		edge.Annotation{
			// don't serialize edges to JSON
			StructTag: `json:"-"`,
		},
	}
}

// NOTE: field comments don't actually go anywhere right now

func (Topic) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).StorageKey("id").Default(uuid.New).Immutable(),
		field.String("name").
			StorageKey("name").
			SchemaType(textTypes).
			Immutable().
			NotEmpty(),
		field.Time("createdAt").
			StorageKey("created_at").
			SchemaType(timestampTypes).
			Default(time.Now).
			Immutable(),
		// combination of live & deleted markers, plus some constraints and indexes,
		// give us db-level protection against duplicate topics
		field.Bool("live").
			StorageKey("live").
			Nillable().
			Optional().
			Default(true),
		field.Time("deletedAt").
			StorageKey("deleted_at").
			SchemaType(timestampTypes).
			Optional().
			Nillable(),
		field.JSON("labels", map[string]string{}).
			StorageKey("labels").
			SchemaType(jsonTypes).
			Optional(),
	}
}

func (Topic) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("subscriptions", Subscription.Type).
			Ref("topic"),
		// TODO: this is a dangerous relation to expose as it will produce enormous
		// amounts of data
		edge.From("messages", Message.Type).
			Ref("topic"),
	}
}

func (Topic) Indexes() []ent.Index {
	return []ent.Index{
		// nulls are distinct in a unique index
		index.Fields("name", "live").
			Unique(),
		index.Fields("deletedAt"),
	}
}

func (Topic) Hooks() []ent.Hook {
	return []ent.Hook{
		wrapHook(checkLiveOrDeleted),
	}
}

func (Subscription) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("id").
			Default(uuid.New).
			Immutable(),
		// topicID should be immutable, but that breaks the edge
		field.UUID("topicID", uuid.UUID{}).
			StorageKey("topic_id"),
		field.String("name").
			StorageKey("name").
			SchemaType(textTypes).
			Immutable().
			NotEmpty(),
		field.Time("createdAt").
			StorageKey("created_at").
			SchemaType(timestampTypes).
			Default(time.Now).
			Immutable(),
		field.Time("expiresAt").
			StorageKey("expires_at").
			SchemaType(timestampTypes),
		// combination of live & deleted markers, plus some constraints and indexes,
		// give us db-level protection against duplicate topics
		field.Bool("live").
			StorageKey("live").
			Nillable().
			Optional().
			Default(true),
		field.Time("deletedAt").
			StorageKey("deleted_at").
			SchemaType(timestampTypes).
			Optional().
			Nillable(),
		// https://github.com/ent/ent/issues/1168
		// PG understands Go interval format, but Go doesn't understand PG's
		field.String("ttl").
			StorageKey("ttl").
			SchemaType(intervalTypes).
			GoType((*customtypes.Interval)(nil)),
		field.String("messageTTL").
			StorageKey("message_ttl").
			SchemaType(intervalTypes).
			GoType((*customtypes.Interval)(nil)),
		field.Bool("orderedDelivery").
			StorageKey("ordered_delivery").
			Optional().
			Default(false),
		field.JSON("labels", map[string]string{}).
			StorageKey("labels").
			SchemaType(jsonTypes).
			Optional(),
		field.String("minBackoff").
			StorageKey("min_backoff").
			SchemaType(intervalTypes).
			GoType((*customtypes.IntervalNull)(nil)).
			Optional().
			Nillable(),
		field.String("maxBackoff").
			StorageKey("max_backoff").
			SchemaType(intervalTypes).
			GoType((*customtypes.IntervalNull)(nil)).
			Optional().
			Nillable(),
		field.String("pushEndpoint").
			StorageKey("push_endpoint").
			Optional().
			Nillable(),
		field.String("messageFilter").
			StorageKey("filter").
			Optional().
			Nillable(),
	}
}

func (Subscription) Edges() []ent.Edge {
	return []ent.Edge{
		// topic is NOT required -- a subscription can be detached from a topic, at
		// which point it will not receive new messages, but existing messages can
		// still be read out
		edge.To("topic", Topic.Type).
			Unique().
			Field("topicID").
			Required().
			StorageKey(edge.Column("topic_id")),
		edge.From("deliveries", Delivery.Type).
			Ref("subscription"),
	}
}

func (Subscription) Indexes() []ent.Index {
	return []ent.Index{
		// nulls are distinct in a unique index
		index.Fields("name", "live").
			Unique(),
		index.Fields("deletedAt"),
	}
}

func (Subscription) Hooks() []ent.Hook {
	return []ent.Hook{
		wrapHook(checkLiveOrDeleted),
	}
}

func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("id").
			Default(uuid.New).
			Immutable(),
		// topicID should be immutable, but that breaks the edge
		field.UUID("topicID", uuid.UUID{}).
			StorageKey("topic_id"),
		// TODO: re-evaluate if we want to limit this to just storing JSON payloads
		field.JSON("payload", json.RawMessage{}).
			SchemaType(jsonTypes).
			Immutable(),
		field.JSON("attributes", map[string]string{}).
			StorageKey("attributes").
			SchemaType(jsonTypes).
			Optional().
			Immutable(),
		field.Time("publishedAt").
			StorageKey("published_at").
			SchemaType(timestampTypes).
			Default(time.Now).
			Immutable(),
		field.String("orderKey").
			StorageKey("order_key").
			SchemaType(textTypes).
			Optional().
			Nillable().
			Immutable(),
	}
}

func (Message) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("deliveries", Delivery.Type).
			Ref("message"),
		// making the topic edge required means we can't fully delete a topic until
		// all messages are deleted, which means that topic delete starts out as a
		// soft delete.
		// TODO: we'd like this edge to be immutable, but that's not supported
		edge.To("topic", Topic.Type).
			Unique().
			Field("topicID").
			StorageKey(edge.Column("topic_id")).
			Required(),
	}
}

func (Message) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("publishedAt"),
	}
}

func (Delivery) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			StorageKey("id").
			Default(uuid.New).
			Immutable(),
		// messageID should be immutable, but that breaks the edge
		field.UUID("messageID", uuid.UUID{}).
			StorageKey("message_id"),
		// subscriptionID should be immutable, but that breaks the edge
		field.UUID("subscriptionID", uuid.UUID{}).
			StorageKey("subscription_id"),
		field.Time("publishedAt").
			StorageKey("published_at").
			SchemaType(timestampTypes).
			Default(time.Now).
			Comment("Copy of message.publishedAt for ordered delivery support"),
		// TODO: possibly a (db-generated) sequenceId for ordering, instead of using publishedAt?
		field.Time("attemptAt").
			StorageKey("attempt_at").
			SchemaType(timestampTypes).
			Default(time.Now).
			Comment("Earliest time at which delivery should next be attempted"),
		field.Time("lastAttemptedAt").
			StorageKey("last_attempted_at").
			SchemaType(timestampTypes).
			Optional().
			Nillable().
			Comment("Time last attempt was started"),
		field.Int("attempts").
			StorageKey("attempts").
			SchemaType(timestampTypes).
			Default(0).
			Comment("Number of attempts started"),
		field.Time("completedAt").
			StorageKey("completed_at").
			SchemaType(timestampTypes).
			Optional().
			Nillable().
			Comment("Time when last successfully delivered, or NULL if not yet"),
		field.Time("expiresAt").
			StorageKey("expires_at").
			SchemaType(timestampTypes).
			Comment("Time beyond which delivery should no longer be attempted even if not successful"),
		field.UUID("notBeforeID", uuid.UUID{}).
			StorageKey("not_before_id").
			Optional(),
	}
}

func (Delivery) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("message", Message.Type).
			Unique().
			Field("messageID").
			StorageKey(edge.Column("message_id")).
			Required(),
		edge.To("subscription", Subscription.Type).
			Unique().
			Field("subscriptionID").
			StorageKey(edge.Column("subscription_id")).
			Required(),
		edge.To("nextReady", Delivery.Type).
			From("notBefore").
			Field("notBeforeID").
			Unique(),
	}
}

func (Delivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("publishedAt"),
		index.Fields("attemptAt"),
		index.Fields("expiresAt"),
		index.Edges("notBefore"),
		index.Edges("subscription"),
		index.Edges("message"),
	}
}

func wrapHook(f func(context.Context, ent.Mutator, ent.Mutation) (ent.Value, error)) ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			return f(ctx, next, m)
		})
	}
}

func checkLiveOrDeleted(ctx context.Context, next ent.Mutator, m ent.Mutation) (ent.Value, error) {
	if m.Op().Is(ent.OpDelete | ent.OpDeleteOne) {
		// this hook is only to protect create/update
		return next.Mutate(ctx, m)
	}
	// this validation is handled by a check constraint in PostgreSQL.
	// SQLite supports the check constraint syntax too, but ent doesn't
	// allow us to specify check constraints in the schema.
	var live *bool
	if liveI, setLive := m.Field("live"); setLive {
		l := liveI.(bool)
		live = &l
	}
	clearedLive := m.FieldCleared("live")
	// live is optional, but defaults to true, so we always expect to see it
	// set on create
	deletedAt, setDeletedAt := m.Field("deleted_at")
	clearedDeletedAt := m.FieldCleared("deleted_at")
	if !setDeletedAt && !clearedDeletedAt && m.Op().Is(ent.OpCreate) {
		// DeletedAt is optional, so implicitly nil on create if not explicitly set
		clearedDeletedAt = true
	}

	if live != nil && !*live {
		return nil, errors.New("topic.live can only be set to null or true, not false")
	}
	if setDeletedAt && deletedAt != nil && !clearedLive {
		return nil, errors.New("topic.live must be cleared when setting deletedAt")
	}
	if clearedLive && (!setDeletedAt || deletedAt == nil) {
		return nil, errors.New("topic.deletedAt must be set when clearing live")
	}
	// clearing deletedAt is not normal, but not prohibited
	if clearedDeletedAt && (live == nil || !*live) {
		return nil, errors.New("topic.live must be set true when clearing deletedAt")
	}
	if live != nil && *live && !clearedDeletedAt {
		return nil, errors.New("topic.deletedAt must be cleared when setting live true")
	}

	return next.Mutate(ctx, m)
}
