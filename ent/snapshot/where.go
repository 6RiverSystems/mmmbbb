// Code generated by ent, DO NOT EDIT.

package snapshot

import (
	"time"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/predicate"
)

// ID filters vertices based on their ID field.
func ID(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldID, id))
}

// IDEQ applies the EQ predicate on the ID field.
func IDEQ(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldID, id))
}

// IDNEQ applies the NEQ predicate on the ID field.
func IDNEQ(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldID, id))
}

// IDIn applies the In predicate on the ID field.
func IDIn(ids ...uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldID, ids...))
}

// IDNotIn applies the NotIn predicate on the ID field.
func IDNotIn(ids ...uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldID, ids...))
}

// IDGT applies the GT predicate on the ID field.
func IDGT(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGT(FieldID, id))
}

// IDGTE applies the GTE predicate on the ID field.
func IDGTE(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGTE(FieldID, id))
}

// IDLT applies the LT predicate on the ID field.
func IDLT(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLT(FieldID, id))
}

// IDLTE applies the LTE predicate on the ID field.
func IDLTE(id uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLTE(FieldID, id))
}

// TopicID applies equality check predicate on the "topicID" field. It's identical to TopicIDEQ.
func TopicID(v uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldTopicID, v))
}

// Name applies equality check predicate on the "name" field. It's identical to NameEQ.
func Name(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldName, v))
}

// CreatedAt applies equality check predicate on the "createdAt" field. It's identical to CreatedAtEQ.
func CreatedAt(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldCreatedAt, v))
}

// ExpiresAt applies equality check predicate on the "expiresAt" field. It's identical to ExpiresAtEQ.
func ExpiresAt(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldExpiresAt, v))
}

// AckedMessagesBefore applies equality check predicate on the "ackedMessagesBefore" field. It's identical to AckedMessagesBeforeEQ.
func AckedMessagesBefore(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldAckedMessagesBefore, v))
}

// TopicIDEQ applies the EQ predicate on the "topicID" field.
func TopicIDEQ(v uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldTopicID, v))
}

// TopicIDNEQ applies the NEQ predicate on the "topicID" field.
func TopicIDNEQ(v uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldTopicID, v))
}

// TopicIDIn applies the In predicate on the "topicID" field.
func TopicIDIn(vs ...uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldTopicID, vs...))
}

// TopicIDNotIn applies the NotIn predicate on the "topicID" field.
func TopicIDNotIn(vs ...uuid.UUID) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldTopicID, vs...))
}

// NameEQ applies the EQ predicate on the "name" field.
func NameEQ(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldName, v))
}

// NameNEQ applies the NEQ predicate on the "name" field.
func NameNEQ(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldName, v))
}

// NameIn applies the In predicate on the "name" field.
func NameIn(vs ...string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldName, vs...))
}

// NameNotIn applies the NotIn predicate on the "name" field.
func NameNotIn(vs ...string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldName, vs...))
}

// NameGT applies the GT predicate on the "name" field.
func NameGT(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGT(FieldName, v))
}

// NameGTE applies the GTE predicate on the "name" field.
func NameGTE(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGTE(FieldName, v))
}

// NameLT applies the LT predicate on the "name" field.
func NameLT(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLT(FieldName, v))
}

// NameLTE applies the LTE predicate on the "name" field.
func NameLTE(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLTE(FieldName, v))
}

// NameContains applies the Contains predicate on the "name" field.
func NameContains(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldContains(FieldName, v))
}

// NameHasPrefix applies the HasPrefix predicate on the "name" field.
func NameHasPrefix(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldHasPrefix(FieldName, v))
}

// NameHasSuffix applies the HasSuffix predicate on the "name" field.
func NameHasSuffix(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldHasSuffix(FieldName, v))
}

// NameEqualFold applies the EqualFold predicate on the "name" field.
func NameEqualFold(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEqualFold(FieldName, v))
}

// NameContainsFold applies the ContainsFold predicate on the "name" field.
func NameContainsFold(v string) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldContainsFold(FieldName, v))
}

// CreatedAtEQ applies the EQ predicate on the "createdAt" field.
func CreatedAtEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldCreatedAt, v))
}

// CreatedAtNEQ applies the NEQ predicate on the "createdAt" field.
func CreatedAtNEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldCreatedAt, v))
}

// CreatedAtIn applies the In predicate on the "createdAt" field.
func CreatedAtIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldCreatedAt, vs...))
}

// CreatedAtNotIn applies the NotIn predicate on the "createdAt" field.
func CreatedAtNotIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldCreatedAt, vs...))
}

// CreatedAtGT applies the GT predicate on the "createdAt" field.
func CreatedAtGT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGT(FieldCreatedAt, v))
}

// CreatedAtGTE applies the GTE predicate on the "createdAt" field.
func CreatedAtGTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGTE(FieldCreatedAt, v))
}

// CreatedAtLT applies the LT predicate on the "createdAt" field.
func CreatedAtLT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLT(FieldCreatedAt, v))
}

// CreatedAtLTE applies the LTE predicate on the "createdAt" field.
func CreatedAtLTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLTE(FieldCreatedAt, v))
}

// ExpiresAtEQ applies the EQ predicate on the "expiresAt" field.
func ExpiresAtEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldExpiresAt, v))
}

// ExpiresAtNEQ applies the NEQ predicate on the "expiresAt" field.
func ExpiresAtNEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldExpiresAt, v))
}

// ExpiresAtIn applies the In predicate on the "expiresAt" field.
func ExpiresAtIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldExpiresAt, vs...))
}

// ExpiresAtNotIn applies the NotIn predicate on the "expiresAt" field.
func ExpiresAtNotIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldExpiresAt, vs...))
}

// ExpiresAtGT applies the GT predicate on the "expiresAt" field.
func ExpiresAtGT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGT(FieldExpiresAt, v))
}

// ExpiresAtGTE applies the GTE predicate on the "expiresAt" field.
func ExpiresAtGTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGTE(FieldExpiresAt, v))
}

// ExpiresAtLT applies the LT predicate on the "expiresAt" field.
func ExpiresAtLT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLT(FieldExpiresAt, v))
}

// ExpiresAtLTE applies the LTE predicate on the "expiresAt" field.
func ExpiresAtLTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLTE(FieldExpiresAt, v))
}

// LabelsIsNil applies the IsNil predicate on the "labels" field.
func LabelsIsNil() predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIsNull(FieldLabels))
}

// LabelsNotNil applies the NotNil predicate on the "labels" field.
func LabelsNotNil() predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotNull(FieldLabels))
}

// AckedMessagesBeforeEQ applies the EQ predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldEQ(FieldAckedMessagesBefore, v))
}

// AckedMessagesBeforeNEQ applies the NEQ predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeNEQ(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNEQ(FieldAckedMessagesBefore, v))
}

// AckedMessagesBeforeIn applies the In predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIn(FieldAckedMessagesBefore, vs...))
}

// AckedMessagesBeforeNotIn applies the NotIn predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeNotIn(vs ...time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotIn(FieldAckedMessagesBefore, vs...))
}

// AckedMessagesBeforeGT applies the GT predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeGT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGT(FieldAckedMessagesBefore, v))
}

// AckedMessagesBeforeGTE applies the GTE predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeGTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldGTE(FieldAckedMessagesBefore, v))
}

// AckedMessagesBeforeLT applies the LT predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeLT(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLT(FieldAckedMessagesBefore, v))
}

// AckedMessagesBeforeLTE applies the LTE predicate on the "ackedMessagesBefore" field.
func AckedMessagesBeforeLTE(v time.Time) predicate.Snapshot {
	return predicate.Snapshot(sql.FieldLTE(FieldAckedMessagesBefore, v))
}

// AckedMessageIDsIsNil applies the IsNil predicate on the "ackedMessageIDs" field.
func AckedMessageIDsIsNil() predicate.Snapshot {
	return predicate.Snapshot(sql.FieldIsNull(FieldAckedMessageIDs))
}

// AckedMessageIDsNotNil applies the NotNil predicate on the "ackedMessageIDs" field.
func AckedMessageIDsNotNil() predicate.Snapshot {
	return predicate.Snapshot(sql.FieldNotNull(FieldAckedMessageIDs))
}

// HasTopic applies the HasEdge predicate on the "topic" edge.
func HasTopic() predicate.Snapshot {
	return predicate.Snapshot(func(s *sql.Selector) {
		step := sqlgraph.NewStep(
			sqlgraph.From(Table, FieldID),
			sqlgraph.Edge(sqlgraph.M2O, false, TopicTable, TopicColumn),
		)
		sqlgraph.HasNeighbors(s, step)
	})
}

// HasTopicWith applies the HasEdge predicate on the "topic" edge with a given conditions (other predicates).
func HasTopicWith(preds ...predicate.Topic) predicate.Snapshot {
	return predicate.Snapshot(func(s *sql.Selector) {
		step := newTopicStep()
		sqlgraph.HasNeighborsWith(s, step, func(s *sql.Selector) {
			for _, p := range preds {
				p(s)
			}
		})
	})
}

// And groups predicates with the AND operator between them.
func And(predicates ...predicate.Snapshot) predicate.Snapshot {
	return predicate.Snapshot(sql.AndPredicates(predicates...))
}

// Or groups predicates with the OR operator between them.
func Or(predicates ...predicate.Snapshot) predicate.Snapshot {
	return predicate.Snapshot(sql.OrPredicates(predicates...))
}

// Not applies the not operator on the given predicate.
func Not(p predicate.Snapshot) predicate.Snapshot {
	return predicate.Snapshot(sql.NotPredicates(p))
}
