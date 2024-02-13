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

	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/topic"
)

// Snapshot is the model entity for the Snapshot schema.
type Snapshot struct {
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
	// Labels holds the value of the "labels" field.
	Labels map[string]string `json:"labels,omitempty"`
	// AckedMessagesBefore holds the value of the "ackedMessagesBefore" field.
	AckedMessagesBefore time.Time `json:"ackedMessagesBefore,omitempty"`
	// AckedMessageIDs holds the value of the "ackedMessageIDs" field.
	AckedMessageIDs []uuid.UUID `json:"ackedMessageIDs,omitempty"`
	// Edges holds the relations/edges for other nodes in the graph.
	// The values are being populated by the SnapshotQuery when eager-loading is set.
	Edges        SnapshotEdges `json:"-"`
	selectValues sql.SelectValues
}

// SnapshotEdges holds the relations/edges for other nodes in the graph.
type SnapshotEdges struct {
	// Topic holds the value of the topic edge.
	Topic *Topic `json:"topic,omitempty"`
	// loadedTypes holds the information for reporting if a
	// type was loaded (or requested) in eager-loading or not.
	loadedTypes [1]bool
}

// TopicOrErr returns the Topic value or an error if the edge
// was not loaded in eager-loading, or loaded but was not found.
func (e SnapshotEdges) TopicOrErr() (*Topic, error) {
	if e.loadedTypes[0] {
		if e.Topic == nil {
			// Edge was loaded but was not found.
			return nil, &NotFoundError{label: topic.Label}
		}
		return e.Topic, nil
	}
	return nil, &NotLoadedError{edge: "topic"}
}

// scanValues returns the types for scanning values from sql.Rows.
func (*Snapshot) scanValues(columns []string) ([]any, error) {
	values := make([]any, len(columns))
	for i := range columns {
		switch columns[i] {
		case snapshot.FieldLabels, snapshot.FieldAckedMessageIDs:
			values[i] = new([]byte)
		case snapshot.FieldName:
			values[i] = new(sql.NullString)
		case snapshot.FieldCreatedAt, snapshot.FieldExpiresAt, snapshot.FieldAckedMessagesBefore:
			values[i] = new(sql.NullTime)
		case snapshot.FieldID, snapshot.FieldTopicID:
			values[i] = new(uuid.UUID)
		default:
			values[i] = new(sql.UnknownType)
		}
	}
	return values, nil
}

// assignValues assigns the values that were returned from sql.Rows (after scanning)
// to the Snapshot fields.
func (s *Snapshot) assignValues(columns []string, values []any) error {
	if m, n := len(values), len(columns); m < n {
		return fmt.Errorf("mismatch number of scan values: %d != %d", m, n)
	}
	for i := range columns {
		switch columns[i] {
		case snapshot.FieldID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field id", values[i])
			} else if value != nil {
				s.ID = *value
			}
		case snapshot.FieldTopicID:
			if value, ok := values[i].(*uuid.UUID); !ok {
				return fmt.Errorf("unexpected type %T for field topicID", values[i])
			} else if value != nil {
				s.TopicID = *value
			}
		case snapshot.FieldName:
			if value, ok := values[i].(*sql.NullString); !ok {
				return fmt.Errorf("unexpected type %T for field name", values[i])
			} else if value.Valid {
				s.Name = value.String
			}
		case snapshot.FieldCreatedAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field createdAt", values[i])
			} else if value.Valid {
				s.CreatedAt = value.Time
			}
		case snapshot.FieldExpiresAt:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field expiresAt", values[i])
			} else if value.Valid {
				s.ExpiresAt = value.Time
			}
		case snapshot.FieldLabels:
			if value, ok := values[i].(*[]byte); !ok {
				return fmt.Errorf("unexpected type %T for field labels", values[i])
			} else if value != nil && len(*value) > 0 {
				if err := json.Unmarshal(*value, &s.Labels); err != nil {
					return fmt.Errorf("unmarshal field labels: %w", err)
				}
			}
		case snapshot.FieldAckedMessagesBefore:
			if value, ok := values[i].(*sql.NullTime); !ok {
				return fmt.Errorf("unexpected type %T for field ackedMessagesBefore", values[i])
			} else if value.Valid {
				s.AckedMessagesBefore = value.Time
			}
		case snapshot.FieldAckedMessageIDs:
			if value, ok := values[i].(*[]byte); !ok {
				return fmt.Errorf("unexpected type %T for field ackedMessageIDs", values[i])
			} else if value != nil && len(*value) > 0 {
				if err := json.Unmarshal(*value, &s.AckedMessageIDs); err != nil {
					return fmt.Errorf("unmarshal field ackedMessageIDs: %w", err)
				}
			}
		default:
			s.selectValues.Set(columns[i], values[i])
		}
	}
	return nil
}

// Value returns the ent.Value that was dynamically selected and assigned to the Snapshot.
// This includes values selected through modifiers, order, etc.
func (s *Snapshot) Value(name string) (ent.Value, error) {
	return s.selectValues.Get(name)
}

// QueryTopic queries the "topic" edge of the Snapshot entity.
func (s *Snapshot) QueryTopic() *TopicQuery {
	return NewSnapshotClient(s.config).QueryTopic(s)
}

// Update returns a builder for updating this Snapshot.
// Note that you need to call Snapshot.Unwrap() before calling this method if this Snapshot
// was returned from a transaction, and the transaction was committed or rolled back.
func (s *Snapshot) Update() *SnapshotUpdateOne {
	return NewSnapshotClient(s.config).UpdateOne(s)
}

// Unwrap unwraps the Snapshot entity that was returned from a transaction after it was closed,
// so that all future queries will be executed through the driver which created the transaction.
func (s *Snapshot) Unwrap() *Snapshot {
	_tx, ok := s.config.driver.(*txDriver)
	if !ok {
		panic("ent: Snapshot is not a transactional entity")
	}
	s.config.driver = _tx.drv
	return s
}

// String implements the fmt.Stringer.
func (s *Snapshot) String() string {
	var builder strings.Builder
	builder.WriteString("Snapshot(")
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
	builder.WriteString("labels=")
	builder.WriteString(fmt.Sprintf("%v", s.Labels))
	builder.WriteString(", ")
	builder.WriteString("ackedMessagesBefore=")
	builder.WriteString(s.AckedMessagesBefore.Format(time.ANSIC))
	builder.WriteString(", ")
	builder.WriteString("ackedMessageIDs=")
	builder.WriteString(fmt.Sprintf("%v", s.AckedMessageIDs))
	builder.WriteByte(')')
	return builder.String()
}

// Snapshots is a parsable slice of Snapshot.
type Snapshots []*Snapshot