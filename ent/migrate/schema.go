// Code generated by ent, DO NOT EDIT.

package migrate

import (
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/dialect/sql/schema"
	"entgo.io/ent/schema/field"
)

var (
	// DeliveriesColumns holds the columns for the "deliveries" table.
	DeliveriesColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "published_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "attempt_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "last_attempted_at", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "attempts", Type: field.TypeInt, Default: 0, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "completed_at", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "expires_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "message_id", Type: field.TypeUUID},
		{Name: "subscription_id", Type: field.TypeUUID},
		{Name: "not_before_id", Type: field.TypeUUID, Nullable: true},
	}
	// DeliveriesTable holds the schema information for the "deliveries" table.
	DeliveriesTable = &schema.Table{
		Name:       "deliveries",
		Columns:    DeliveriesColumns,
		PrimaryKey: []*schema.Column{DeliveriesColumns[0]},
		ForeignKeys: []*schema.ForeignKey{
			{
				Symbol:     "deliveries_messages_message",
				Columns:    []*schema.Column{DeliveriesColumns[7]},
				RefColumns: []*schema.Column{MessagesColumns[0]},
				OnDelete:   schema.NoAction,
			},
			{
				Symbol:     "deliveries_subscriptions_subscription",
				Columns:    []*schema.Column{DeliveriesColumns[8]},
				RefColumns: []*schema.Column{SubscriptionsColumns[0]},
				OnDelete:   schema.NoAction,
			},
			{
				Symbol:     "deliveries_deliveries_nextReady",
				Columns:    []*schema.Column{DeliveriesColumns[9]},
				RefColumns: []*schema.Column{DeliveriesColumns[0]},
				OnDelete:   schema.SetNull,
			},
		},
		Indexes: []*schema.Index{
			{
				Name:    "delivery_published_at",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[1]},
			},
			{
				Name:    "delivery_attempt_at",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[2]},
			},
			{
				Name:    "delivery_expires_at",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[6]},
			},
			{
				Name:    "delivery_not_before_id",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[9]},
			},
			{
				Name:    "delivery_subscription_id",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[8]},
			},
			{
				Name:    "delivery_message_id",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[7]},
			},
			{
				Name:    "delivery_subscription_id_attempt_at",
				Unique:  false,
				Columns: []*schema.Column{DeliveriesColumns[8], DeliveriesColumns[2]},
				Annotation: &entsql.IndexAnnotation{
					Where: "completed_at is null",
				},
			},
		},
	}
	// MessagesColumns holds the columns for the "messages" table.
	MessagesColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "payload", Type: field.TypeJSON, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
		{Name: "attributes", Type: field.TypeJSON, Nullable: true, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
		{Name: "published_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "order_key", Type: field.TypeString, Nullable: true, SchemaType: map[string]string{"postgres": "text", "sqlite3": "text"}},
		{Name: "topic_id", Type: field.TypeUUID},
	}
	// MessagesTable holds the schema information for the "messages" table.
	MessagesTable = &schema.Table{
		Name:       "messages",
		Columns:    MessagesColumns,
		PrimaryKey: []*schema.Column{MessagesColumns[0]},
		ForeignKeys: []*schema.ForeignKey{
			{
				Symbol:     "messages_topics_topic",
				Columns:    []*schema.Column{MessagesColumns[5]},
				RefColumns: []*schema.Column{TopicsColumns[0]},
				OnDelete:   schema.NoAction,
			},
		},
		Indexes: []*schema.Index{
			{
				Name:    "message_published_at",
				Unique:  false,
				Columns: []*schema.Column{MessagesColumns[3]},
			},
		},
	}
	// SnapshotsColumns holds the columns for the "snapshots" table.
	SnapshotsColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "name", Type: field.TypeString, Unique: true, SchemaType: map[string]string{"postgres": "text", "sqlite3": "text"}},
		{Name: "created_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "expires_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "labels", Type: field.TypeJSON, Nullable: true, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
		{Name: "acked_messages_before", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "acked_message_ids", Type: field.TypeJSON, Nullable: true, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
		{Name: "topic_id", Type: field.TypeUUID},
	}
	// SnapshotsTable holds the schema information for the "snapshots" table.
	SnapshotsTable = &schema.Table{
		Name:       "snapshots",
		Columns:    SnapshotsColumns,
		PrimaryKey: []*schema.Column{SnapshotsColumns[0]},
		ForeignKeys: []*schema.ForeignKey{
			{
				Symbol:     "snapshots_topics_topic",
				Columns:    []*schema.Column{SnapshotsColumns[7]},
				RefColumns: []*schema.Column{TopicsColumns[0]},
				OnDelete:   schema.NoAction,
			},
		},
	}
	// SubscriptionsColumns holds the columns for the "subscriptions" table.
	SubscriptionsColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "name", Type: field.TypeString, SchemaType: map[string]string{"postgres": "text", "sqlite3": "text"}},
		{Name: "created_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "expires_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "live", Type: field.TypeBool, Nullable: true, Default: true},
		{Name: "deleted_at", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "ttl", Type: field.TypeString, SchemaType: map[string]string{"postgres": "interval", "sqlite3": "text"}},
		{Name: "message_ttl", Type: field.TypeString, SchemaType: map[string]string{"postgres": "interval", "sqlite3": "text"}},
		{Name: "ordered_delivery", Type: field.TypeBool, Nullable: true, Default: false},
		{Name: "labels", Type: field.TypeJSON, Nullable: true, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
		{Name: "min_backoff", Type: field.TypeString, Nullable: true, SchemaType: map[string]string{"postgres": "interval", "sqlite3": "text"}},
		{Name: "max_backoff", Type: field.TypeString, Nullable: true, SchemaType: map[string]string{"postgres": "interval", "sqlite3": "text"}},
		{Name: "push_endpoint", Type: field.TypeString, Nullable: true},
		{Name: "filter", Type: field.TypeString, Nullable: true},
		{Name: "max_delivery_attempts", Type: field.TypeInt32, Nullable: true},
		{Name: "delivery_delay", Type: field.TypeOther, SchemaType: map[string]string{"postgres": "interval", "sqlite3": "text"}},
		{Name: "topic_id", Type: field.TypeUUID},
		{Name: "dead_letter_topic_id", Type: field.TypeUUID, Nullable: true},
	}
	// SubscriptionsTable holds the schema information for the "subscriptions" table.
	SubscriptionsTable = &schema.Table{
		Name:       "subscriptions",
		Columns:    SubscriptionsColumns,
		PrimaryKey: []*schema.Column{SubscriptionsColumns[0]},
		ForeignKeys: []*schema.ForeignKey{
			{
				Symbol:     "subscriptions_topics_topic",
				Columns:    []*schema.Column{SubscriptionsColumns[16]},
				RefColumns: []*schema.Column{TopicsColumns[0]},
				OnDelete:   schema.NoAction,
			},
			{
				Symbol:     "subscriptions_topics_deadLetterTopic",
				Columns:    []*schema.Column{SubscriptionsColumns[17]},
				RefColumns: []*schema.Column{TopicsColumns[0]},
				OnDelete:   schema.SetNull,
			},
		},
		Indexes: []*schema.Index{
			{
				Name:    "subscription_name_live",
				Unique:  true,
				Columns: []*schema.Column{SubscriptionsColumns[1], SubscriptionsColumns[4]},
			},
			{
				Name:    "subscription_deleted_at",
				Unique:  false,
				Columns: []*schema.Column{SubscriptionsColumns[5]},
			},
		},
	}
	// TopicsColumns holds the columns for the "topics" table.
	TopicsColumns = []*schema.Column{
		{Name: "id", Type: field.TypeUUID},
		{Name: "name", Type: field.TypeString, SchemaType: map[string]string{"postgres": "text", "sqlite3": "text"}},
		{Name: "created_at", Type: field.TypeTime, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "live", Type: field.TypeBool, Nullable: true, Default: true},
		{Name: "deleted_at", Type: field.TypeTime, Nullable: true, SchemaType: map[string]string{"postgres": "timestamptz"}},
		{Name: "labels", Type: field.TypeJSON, Nullable: true, SchemaType: map[string]string{"postgres": "jsonb", "sqlite3": "json"}},
	}
	// TopicsTable holds the schema information for the "topics" table.
	TopicsTable = &schema.Table{
		Name:       "topics",
		Columns:    TopicsColumns,
		PrimaryKey: []*schema.Column{TopicsColumns[0]},
		Indexes: []*schema.Index{
			{
				Name:    "topic_name_live",
				Unique:  true,
				Columns: []*schema.Column{TopicsColumns[1], TopicsColumns[3]},
			},
			{
				Name:    "topic_deleted_at",
				Unique:  false,
				Columns: []*schema.Column{TopicsColumns[4]},
			},
		},
	}
	// Tables holds all the tables in the schema.
	Tables = []*schema.Table{
		DeliveriesTable,
		MessagesTable,
		SnapshotsTable,
		SubscriptionsTable,
		TopicsTable,
	}
)

func init() {
	DeliveriesTable.ForeignKeys[0].RefTable = MessagesTable
	DeliveriesTable.ForeignKeys[1].RefTable = SubscriptionsTable
	DeliveriesTable.ForeignKeys[2].RefTable = DeliveriesTable
	DeliveriesTable.Annotation = &entsql.Annotation{
		Table: "deliveries",
	}
	MessagesTable.ForeignKeys[0].RefTable = TopicsTable
	MessagesTable.Annotation = &entsql.Annotation{
		Table: "messages",
	}
	SnapshotsTable.ForeignKeys[0].RefTable = TopicsTable
	SnapshotsTable.Annotation = &entsql.Annotation{
		Table: "snapshots",
	}
	SubscriptionsTable.ForeignKeys[0].RefTable = TopicsTable
	SubscriptionsTable.ForeignKeys[1].RefTable = TopicsTable
	SubscriptionsTable.Annotation = &entsql.Annotation{
		Table: "subscriptions",
	}
	TopicsTable.Annotation = &entsql.Annotation{
		Table: "topics",
	}
}
