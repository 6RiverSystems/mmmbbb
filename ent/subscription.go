// Code generated by ent, DO NOT EDIT.

package ent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/internal/sqltypes"
)

// Subscription is the model entity for the Subscription schema.
type Subscription struct {
	config `json:"-"`
	// ID of the ent.
	ID uuid.UUID `json:"id,omitempty"`
	// TopicID holds the value of the "topicID" field.
	TopicID uuid.UUID `json:"topicID,omitempty"`
	// Name holds the value of the "name" field.
	Name string `json:"name,omitempty"`
	// CreatedAt holds the value of the "createdAt" field.
	CreatedAt time.Time `json:"createdAt,omitempty"`
	// ExpiresAt holds the value of the "expiresAt" field.
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	// Live holds the value of the "live" field.
	Live *bool `json:"live,omitempty"`
	// DeletedAt holds the value of the "deletedAt" field.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
	// TTL holds the value of the "ttl" field.
	TTL sqltypes.Interval `json:"ttl,omitempty"`
	// MessageTTL holds the value of the "messageTTL" field.
	MessageTTL sqltypes.Interval `json:"messageTTL,omitempty"`
	// OrderedDelivery holds the value of the "orderedDelivery" field.
	OrderedDelivery bool `json:"orderedDelivery,omitempty"`
	// Labels holds the value of the "labels" field.
	Labels map[string]string `json:"labels,omitempty"`
	// MinBackoff holds the value of the "minBackoff" field.
	MinBackoff *sqltypes.Interval `json:"minBackoff,omitempty"`
	// MaxBackoff holds the value of the "maxBackoff" field.
	MaxBackoff *sqltypes.Interval `json:"maxBackoff,omitempty"`
	// PushEndpoint holds the value of the "pushEndpoint" field.
	PushEndpoint *string `json:"pushEndpoint,omitempty"`
	// MessageFilter holds the value of the "messageFilter" field.
	MessageFilter *string `json:"messageFilter,omitempty"`
	// MaxDeliveryAttempts holds the value of the "maxDeliveryAttempts" field.
	MaxDeliveryAttempts *int32 `json:"maxDeliveryAttempts,omitempty"`
	// DeadLetterTopicID holds the value of the "deadLetterTopicID" field.
	DeadLetterTopicID *uuid.UUID `json:"deadLetterTopicID,omitempty"`
	// DeliveryDelay holds the value of the "deliveryDelay" field.
	DeliveryDelay sqltypes.Interval `json:"deliveryDelay,omitempty"`
	// Edges holds the relations/edges for other nodes in the graph.
	// The values are being populated by the SubscriptionQuery when eager-loading is set.
	Edges        SubscriptionEdges `json:"-"`
	selectValues sql.SelectValues
}

// SubscriptionEdges holds the relations/edges for other nodes in the graph.
type SubscriptionEdges struct {
	// Topic holds the value of the topic edge.
	Topic *Topic `json:"topic,omitempty"`
	// Deliveries holds the value of the deliveries edge.
	Deliveries []*Delivery `json:"deliveries,omitempty"`
	// DeadLetterTopic holds the value of the deadLetterTopic edge.
	DeadLetterTopic *Topic `json:"deadLetterTopic,omitempty"`
	// loadedTypes holds the information for reporting if a
	// type was loaded (or requested) in eager-loading or not.
	loadedTypes [3]bool
}

// TopicOrErr returns the Topic value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e SubscriptionEdges) TopicOrErr() (*Topic, error) {
	if e.Topic != nil {
		return e.Topic, nil
	} else if e.loadedTypes[0] {
		return nil, &NotFoundError{label: topic.Label}
	}
	return nil, &NotLoadedError{edge: "topic"}
}

// DeliveriesOrErr returns the Deliveries value or an error if the edge
// was not loaded in eager-loading.
func (e SubscriptionEdges) DeliveriesOrErr() ([]*Delivery, error) {
	if e.loadedTypes[1] {
		return e.Deliveries, nil
	}
	return nil, &NotLoadedError{edge: "deliveries"}
}

// DeadLetterTopicOrErr returns the DeadLetterTopic value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e SubscriptionEdges) DeadLetterTopicOrErr() (*Topic, error) {
	if e.DeadLetterTopic != nil {
		return e.DeadLetterTopic, nil
	} else if e.loadedTypes[2] {
		return nil, &NotFoundError{label: topic.Label}
	}
	return nil, &NotLoadedError{edge: "deadLetterTopic"}
}

// scanValues returns the types for scanning values from sql.Rows.
func (*Subscription) scanValues(columns []string) ([]any, error) {
	values := make([]any, len(columns))
	for i := range columns {
		switch columns[i] {
		case subscription.FieldMinBackoff, subscription.FieldMaxBackoff:
			values[i] = &sql.NullScanner{S: new(sqltypes.Interval)}
		case subscription.FieldDeadLetterTopicID:
			values[i] = &sql.NullScanner{S: new(uuid.UUID)}
		case subscription.FieldLabels:
			values[i] = new([]byte)
		case subscription.FieldLive, subscription.FieldOrderedDelivery:
			values[i] = new(sql.NullBool)
		case subscription.FieldMaxDeliveryAttempts:
			values[i] = new(sql.NullInt64)
		case subscription.FieldName, subscription.FieldPushEndpoint, subscription.FieldMessageFilter:
			values[i] = new(sql.NullString)
		case subscription.FieldCreatedAt, subscription.FieldExpiresAt, subscription.FieldDeletedAt:
			values[i] = new(sql.NullTime)
		case subscription.FieldTTL, subscription.FieldMessageTTL, subscription.FieldDeliveryDelay:
			values[i] = new(sqltypes.Interval)
		case subscription.FieldID, subscription.FieldTopicID:
			values[i] = new(uuid.UUID)
		default:
			values[i] = new(sql.UnknownType)
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the Subscription fields.
func (s *Subscription) assignValues(columns []string, values []any) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case subscription.FieldID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value != nil {
				s.ID = *value
			}
		case subscription.FieldTopicID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field topicID", values[i])
			} else if value != nil {
				s.TopicID = *value
			}
		case subscription.FieldName:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field name", values[i])
			} else if value.Valid {
				s.Name = value.String
			}
		case subscription.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field createdAt", values[i])
			} else if value.Valid {
				s.CreatedAt = value.Time
			}
		case subscription.FieldExpiresAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field expiresAt", values[i])
			} else if value.Valid {
				s.ExpiresAt = value.Time
			}
		case subscription.FieldLive:
			if value, ok := values[i].(*sql.NullBool); !ok {
				return fmt.Errorf("unexpected type %T for field live", values[i])
			} else if value.Valid {
				s.Live = new(bool)
				*s.Live = value.Bool
			}
		case subscription.FieldDeletedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field deletedAt", values[i])
			} else if value.Valid {
				s.DeletedAt = new(time.Time)
				*s.DeletedAt = value.Time
			}
		case subscription.FieldTTL:
			if value, ok := values[i].(*sqltypes.Interval); !ok {
				return fmt.Errorf("unexpected type %T for field ttl", values[i])
			} else if value != nil {
				s.TTL = *value
			}
		case subscription.FieldMessageTTL:
			if value, ok := values[i].(*sqltypes.Interval); !ok {
				return fmt.Errorf("unexpected type %T for field messageTTL", values[i])
			} else if value != nil {
				s.MessageTTL = *value
			}
		case subscription.FieldOrderedDelivery:
			if value, ok := values[i].(*sql.NullBool); !ok {
				return fmt.Errorf("unexpected type %T for field orderedDelivery", values[i])
			} else if value.Valid {
				s.OrderedDelivery = value.Bool
			}
		case subscription.FieldLabels:
			if value, ok := values[i].(*[]byte); !ok {
				return fmt.Errorf("unexpected type %T for field labels", values[i])
			} else if value != nil && len(*value) > 0 {
				if err := json.Unmarshal(*value, &s.Labels); err != nil {
					return fmt.Errorf("unmarshal field labels: %w", err)
				}
			}
		case subscription.FieldMinBackoff:
			if value, ok := values[i].(*sql.NullScanner); !ok {
				return fmt.Errorf("unexpected type %T for field minBackoff", values[i])
			} else if value.Valid {
				s.MinBackoff = value.S.(*sqltypes.Interval)
			}
		case subscription.FieldMaxBackoff:
			if value, ok := values[i].(*sql.NullScanner); !ok {
				return fmt.Errorf("unexpected type %T for field maxBackoff", values[i])
			} else if value.Valid {
				s.MaxBackoff = value.S.(*sqltypes.Interval)
			}
		case subscription.FieldPushEndpoint:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field pushEndpoint", values[i])
			} else if value.Valid {
				s.PushEndpoint = new(string)
				*s.PushEndpoint = value.String
			}
		case subscription.FieldMessageFilter:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field messageFilter", values[i])
			} else if value.Valid {
				s.MessageFilter = new(string)
				*s.MessageFilter = value.String
			}
		case subscription.FieldMaxDeliveryAttempts:
			if value, ok := values[i].(*sql.NullInt64); !ok {
				return fmt.Errorf("unexpected type %T for field maxDeliveryAttempts", values[i])
			} else if value.Valid {
				s.MaxDeliveryAttempts = new(int32)
				*s.MaxDeliveryAttempts = int32(value.Int64)
			}
		case subscription.FieldDeadLetterTopicID:
			if value, ok := values[i].(*sql.NullScanner); !ok {
				return fmt.Errorf("unexpected type %T for field deadLetterTopicID", values[i])
			} else if value.Valid {
				s.DeadLetterTopicID = new(uuid.UUID)
				*s.DeadLetterTopicID = *value.S.(*uuid.UUID)
			}
		case subscription.FieldDeliveryDelay:
			if value, ok := values[i].(*sqltypes.Interval); !ok {
				return fmt.Errorf("unexpected type %T for field deliveryDelay", values[i])
			} else if value != nil {
				s.DeliveryDelay = *value
			}
		default:
			s.selectValues.Set(columns[i], values[i])
		}
	}
	return nil
}

// Value returns the ent.Value that was dynamically selected and assigned to the Subscription.
// This includes values selected through modifiers, order, etc.
func (s *Subscription) Value(name string) (ent.Value, error) {
	return s.selectValues.Get(name)
}

// QueryTopic queries the "topic" edge of the Subscription entity.
func (s *Subscription) QueryTopic() *TopicQuery {
	return NewSubscriptionClient(s.config).QueryTopic(s)
}

// QueryDeliveries queries the "deliveries" edge of the Subscription entity.
func (s *Subscription) QueryDeliveries() *DeliveryQuery {
	return NewSubscriptionClient(s.config).QueryDeliveries(s)
}

// QueryDeadLetterTopic queries the "deadLetterTopic" edge of the Subscription entity.
func (s *Subscription) QueryDeadLetterTopic() *TopicQuery {
	return NewSubscriptionClient(s.config).QueryDeadLetterTopic(s)
}

// Update returns a builder for updating this Subscription.
// Note that you need to call Subscription.Unwrap() before calling this method if this Subscription
// was returned from a transaction, and the transaction was committed or rolled back.
func (s *Subscription) Update() *SubscriptionUpdateOne {
	return NewSubscriptionClient(s.config).UpdateOne(s)
}

// Unwrap unwraps the Subscription entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (s *Subscription) Unwrap() *Subscription {
	_tx, ok := s.config.driver.(*txDriver)
	if !ok {
		panic("ent: Subscription is not a transactional entity")
	}
	s.config.driver = _tx.drv
	return s
}

// String implements the fmt.Stringer.
func (s *Subscription) String() string {
	var builder strings.Builder
	builder.WriteString("Subscription(")
	builder.WriteString(fmt.Sprintf("id=%v, ", s.ID))
	builder.WriteString("topicID=")
	builder.WriteString(fmt.Sprintf("%v", s.TopicID))
	builder.WriteString(", ")
	builder.WriteString("name=")
	builder.WriteString(s.Name)
	builder.WriteString(", ")
	builder.WriteString("createdAt=")
	builder.WriteString(s.CreatedAt.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("expiresAt=")
	builder.WriteString(s.ExpiresAt.Format(time.ANSIC))
	builder.WriteString(", ")
	if v := s.Live; v != nil {
		builder.WriteString("live=")
		builder.WriteString(fmt.Sprintf("%v", *v))
	}
	builder.WriteString(", ")
	if v := s.DeletedAt; v != nil {
		builder.WriteString("deletedAt=")
		builder.WriteString(v.Format(time.ANSIC))
	}
	builder.WriteString(", ")
	builder.WriteString("ttl=")
	builder.WriteString(fmt.Sprintf("%v", s.TTL))
	builder.WriteString(", ")
	builder.WriteString("messageTTL=")
	builder.WriteString(fmt.Sprintf("%v", s.MessageTTL))
	builder.WriteString(", ")
	builder.WriteString("orderedDelivery=")
	builder.WriteString(fmt.Sprintf("%v", s.OrderedDelivery))
	builder.WriteString(", ")
	builder.WriteString("labels=")
	builder.WriteString(fmt.Sprintf("%v", s.Labels))
	builder.WriteString(", ")
	if v := s.MinBackoff; v != nil {
		builder.WriteString("minBackoff=")
		builder.WriteString(fmt.Sprintf("%v", *v))
	}
	builder.WriteString(", ")
	if v := s.MaxBackoff; v != nil {
		builder.WriteString("maxBackoff=")
		builder.WriteString(fmt.Sprintf("%v", *v))
	}
	builder.WriteString(", ")
	if v := s.PushEndpoint; v != nil {
		builder.WriteString("pushEndpoint=")
		builder.WriteString(*v)
	}
	builder.WriteString(", ")
	if v := s.MessageFilter; v != nil {
		builder.WriteString("messageFilter=")
		builder.WriteString(*v)
	}
	builder.WriteString(", ")
	if v := s.MaxDeliveryAttempts; v != nil {
		builder.WriteString("maxDeliveryAttempts=")
		builder.WriteString(fmt.Sprintf("%v", *v))
	}
	builder.WriteString(", ")
	if v := s.DeadLetterTopicID; v != nil {
		builder.WriteString("deadLetterTopicID=")
		builder.WriteString(fmt.Sprintf("%v", *v))
	}
	builder.WriteString(", ")
	builder.WriteString("deliveryDelay=")
	builder.WriteString(fmt.Sprintf("%v", s.DeliveryDelay))
	builder.WriteByte(')')
	return builder.String()
}

// Subscriptions is a parsable slice of Subscription.
type Subscriptions []*Subscription