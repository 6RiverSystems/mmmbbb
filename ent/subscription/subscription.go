// Code generated by ent, DO NOT EDIT.

package subscription

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/internal/sqltypes"
)

const (
	// Label holds the string label denoting the subscription type in the database.
	Label = "subscription"
	// FieldID holds the string denoting the id field in the database.
	FieldID = "id"
	// FieldTopicID holds the string denoting the topicid field in the database.
	FieldTopicID = "topic_id"
	// FieldName holds the string denoting the name field in the database.
	FieldName = "name"
	// FieldCreatedAt holds the string denoting the createdat field in the database.
	FieldCreatedAt = "created_at"
	// FieldExpiresAt holds the string denoting the expiresat field in the database.
	FieldExpiresAt = "expires_at"
	// FieldLive holds the string denoting the live field in the database.
	FieldLive = "live"
	// FieldDeletedAt holds the string denoting the deletedat field in the database.
	FieldDeletedAt = "deleted_at"
	// FieldTTL holds the string denoting the ttl field in the database.
	FieldTTL = "ttl"
	// FieldMessageTTL holds the string denoting the messagettl field in the database.
	FieldMessageTTL = "message_ttl"
	// FieldOrderedDelivery holds the string denoting the ordereddelivery field in the database.
	FieldOrderedDelivery = "ordered_delivery"
	// FieldLabels holds the string denoting the labels field in the database.
	FieldLabels = "labels"
	// FieldMinBackoff holds the string denoting the minbackoff field in the database.
	FieldMinBackoff = "min_backoff"
	// FieldMaxBackoff holds the string denoting the maxbackoff field in the database.
	FieldMaxBackoff = "max_backoff"
	// FieldPushEndpoint holds the string denoting the pushendpoint field in the database.
	FieldPushEndpoint = "push_endpoint"
	// FieldMessageFilter holds the string denoting the messagefilter field in the database.
	FieldMessageFilter = "filter"
	// FieldMaxDeliveryAttempts holds the string denoting the maxdeliveryattempts field in the database.
	FieldMaxDeliveryAttempts = "max_delivery_attempts"
	// FieldDeadLetterTopicID holds the string denoting the deadlettertopicid field in the database.
	FieldDeadLetterTopicID = "dead_letter_topic_id"
	// FieldDeliveryDelay holds the string denoting the deliverydelay field in the database.
	FieldDeliveryDelay = "delivery_delay"
	// EdgeTopic holds the string denoting the topic edge name in mutations.
	EdgeTopic = "topic"
	// EdgeDeliveries holds the string denoting the deliveries edge name in mutations.
	EdgeDeliveries = "deliveries"
	// EdgeDeadLetterTopic holds the string denoting the deadlettertopic edge name in mutations.
	EdgeDeadLetterTopic = "deadLetterTopic"
	// Table holds the table name of the subscription in the database.
	Table = "subscriptions"
	// TopicTable is the table that holds the topic relation/edge.
	TopicTable = "subscriptions"
	// TopicInverseTable is the table name for the Topic entity.
	// It exists in this package in order to avoid circular dependency with the "topic" package.
	TopicInverseTable = "topics"
	// TopicColumn is the table column denoting the topic relation/edge.
	TopicColumn = "topic_id"
	// DeliveriesTable is the table that holds the deliveries relation/edge.
	DeliveriesTable = "deliveries"
	// DeliveriesInverseTable is the table name for the Delivery entity.
	// It exists in this package in order to avoid circular dependency with the "delivery" package.
	DeliveriesInverseTable = "deliveries"
	// DeliveriesColumn is the table column denoting the deliveries relation/edge.
	DeliveriesColumn = "subscription_id"
	// DeadLetterTopicTable is the table that holds the deadLetterTopic relation/edge.
	DeadLetterTopicTable = "subscriptions"
	// DeadLetterTopicInverseTable is the table name for the Topic entity.
	// It exists in this package in order to avoid circular dependency with the "topic" package.
	DeadLetterTopicInverseTable = "topics"
	// DeadLetterTopicColumn is the table column denoting the deadLetterTopic relation/edge.
	DeadLetterTopicColumn = "dead_letter_topic_id"
)

// Columns holds all SQL columns for subscription fields.
var Columns = []string{
	FieldID,
	FieldTopicID,
	FieldName,
	FieldCreatedAt,
	FieldExpiresAt,
	FieldLive,
	FieldDeletedAt,
	FieldTTL,
	FieldMessageTTL,
	FieldOrderedDelivery,
	FieldLabels,
	FieldMinBackoff,
	FieldMaxBackoff,
	FieldPushEndpoint,
	FieldMessageFilter,
	FieldMaxDeliveryAttempts,
	FieldDeadLetterTopicID,
	FieldDeliveryDelay,
}

// ValidColumn reports if the column name is valid (part of the table columns).
func ValidColumn(column string) bool {
	for i := range Columns {
		if column == Columns[i] {
			return true
		}
	}
	return false
}

// Note that the variables below are initialized by the runtime
// package on the initialization of the application. Therefore,
// it should be imported in the main as follows:
//
//	import _ "go.6river.tech/mmmbbb/ent/runtime"
var (
	Hooks [1]ent.Hook
	// NameValidator is a validator for the "name" field. It is called by the builders before save.
	NameValidator func(string) error
	// DefaultCreatedAt holds the default value on creation for the "createdAt" field.
	DefaultCreatedAt func() time.Time
	// DefaultLive holds the default value on creation for the "live" field.
	DefaultLive bool
	// DefaultOrderedDelivery holds the default value on creation for the "orderedDelivery" field.
	DefaultOrderedDelivery bool
	// DefaultDeliveryDelay holds the default value on creation for the "deliveryDelay" field.
	DefaultDeliveryDelay sqltypes.Interval
	// DefaultID holds the default value on creation for the "id" field.
	DefaultID func() uuid.UUID
)

// OrderOption defines the ordering options for the Subscription queries.
type OrderOption func(*sql.Selector)

// ByID orders the results by the id field.
func ByID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldID, opts...).ToFunc()
}

// ByTopicID orders the results by the topicID field.
func ByTopicID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldTopicID, opts...).ToFunc()
}

// ByName orders the results by the name field.
func ByName(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldName, opts...).ToFunc()
}

// ByCreatedAt orders the results by the createdAt field.
func ByCreatedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldCreatedAt, opts...).ToFunc()
}

// ByExpiresAt orders the results by the expiresAt field.
func ByExpiresAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldExpiresAt, opts...).ToFunc()
}

// ByLive orders the results by the live field.
func ByLive(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldLive, opts...).ToFunc()
}

// ByDeletedAt orders the results by the deletedAt field.
func ByDeletedAt(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldDeletedAt, opts...).ToFunc()
}

// ByTTL orders the results by the ttl field.
func ByTTL(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldTTL, opts...).ToFunc()
}

// ByMessageTTL orders the results by the messageTTL field.
func ByMessageTTL(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldMessageTTL, opts...).ToFunc()
}

// ByOrderedDelivery orders the results by the orderedDelivery field.
func ByOrderedDelivery(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldOrderedDelivery, opts...).ToFunc()
}

// ByMinBackoff orders the results by the minBackoff field.
func ByMinBackoff(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldMinBackoff, opts...).ToFunc()
}

// ByMaxBackoff orders the results by the maxBackoff field.
func ByMaxBackoff(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldMaxBackoff, opts...).ToFunc()
}

// ByPushEndpoint orders the results by the pushEndpoint field.
func ByPushEndpoint(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldPushEndpoint, opts...).ToFunc()
}

// ByMessageFilter orders the results by the messageFilter field.
func ByMessageFilter(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldMessageFilter, opts...).ToFunc()
}

// ByMaxDeliveryAttempts orders the results by the maxDeliveryAttempts field.
func ByMaxDeliveryAttempts(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldMaxDeliveryAttempts, opts...).ToFunc()
}

// ByDeadLetterTopicID orders the results by the deadLetterTopicID field.
func ByDeadLetterTopicID(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldDeadLetterTopicID, opts...).ToFunc()
}

// ByDeliveryDelay orders the results by the deliveryDelay field.
func ByDeliveryDelay(opts ...sql.OrderTermOption) OrderOption {
	return sql.OrderByField(FieldDeliveryDelay, opts...).ToFunc()
}

// ByTopicField orders the results by topic field.
func ByTopicField(field string, opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newTopicStep(), sql.OrderByField(field, opts...))
	}
}

// ByDeliveriesCount orders the results by deliveries count.
func ByDeliveriesCount(opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborsCount(s, newDeliveriesStep(), opts...)
	}
}

// ByDeliveries orders the results by deliveries terms.
func ByDeliveries(term sql.OrderTerm, terms ...sql.OrderTerm) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newDeliveriesStep(), append([]sql.OrderTerm{term}, terms...)...)
	}
}

// ByDeadLetterTopicField orders the results by deadLetterTopic field.
func ByDeadLetterTopicField(field string, opts ...sql.OrderTermOption) OrderOption {
	return func(s *sql.Selector) {
		sqlgraph.OrderByNeighborTerms(s, newDeadLetterTopicStep(), sql.OrderByField(field, opts...))
	}
}
func newTopicStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(TopicInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.M2O, false, TopicTable, TopicColumn),
	)
}
func newDeliveriesStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(DeliveriesInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.O2M, true, DeliveriesTable, DeliveriesColumn),
	)
}
func newDeadLetterTopicStep() *sqlgraph.Step {
	return sqlgraph.NewStep(
		sqlgraph.From(Table, FieldID),
		sqlgraph.To(DeadLetterTopicInverseTable, FieldID),
		sqlgraph.Edge(sqlgraph.M2O, false, DeadLetterTopicTable, DeadLetterTopicColumn),
	)
}
