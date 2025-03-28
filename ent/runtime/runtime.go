// Code generated by ent, DO NOT EDIT.

package runtime

import (
	"time"

	"github.com/google/uuid"
	"go.6river.tech/mmmbbb/ent/delivery"
	"go.6river.tech/mmmbbb/ent/message"
	"go.6river.tech/mmmbbb/ent/schema"
	"go.6river.tech/mmmbbb/ent/snapshot"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/ent/topic"
	"go.6river.tech/mmmbbb/internal/sqltypes"
)

// The init function reads all schema descriptors with runtime code
// (default values, validators, hooks and policies) and stitches it
// to their package variables.
func init() {
	deliveryFields := schema.Delivery{}.Fields()
	_ = deliveryFields
	// deliveryDescPublishedAt is the schema descriptor for publishedAt field.
	deliveryDescPublishedAt := deliveryFields[3].Descriptor()
	// delivery.DefaultPublishedAt holds the default value on creation for the publishedAt field.
	delivery.DefaultPublishedAt = deliveryDescPublishedAt.Default.(func() time.Time)
	// deliveryDescAttemptAt is the schema descriptor for attemptAt field.
	deliveryDescAttemptAt := deliveryFields[4].Descriptor()
	// delivery.DefaultAttemptAt holds the default value on creation for the attemptAt field.
	delivery.DefaultAttemptAt = deliveryDescAttemptAt.Default.(func() time.Time)
	// deliveryDescAttempts is the schema descriptor for attempts field.
	deliveryDescAttempts := deliveryFields[6].Descriptor()
	// delivery.DefaultAttempts holds the default value on creation for the attempts field.
	delivery.DefaultAttempts = deliveryDescAttempts.Default.(int)
	// deliveryDescID is the schema descriptor for id field.
	deliveryDescID := deliveryFields[0].Descriptor()
	// delivery.DefaultID holds the default value on creation for the id field.
	delivery.DefaultID = deliveryDescID.Default.(func() uuid.UUID)
	messageFields := schema.Message{}.Fields()
	_ = messageFields
	// messageDescPublishedAt is the schema descriptor for publishedAt field.
	messageDescPublishedAt := messageFields[4].Descriptor()
	// message.DefaultPublishedAt holds the default value on creation for the publishedAt field.
	message.DefaultPublishedAt = messageDescPublishedAt.Default.(func() time.Time)
	// messageDescID is the schema descriptor for id field.
	messageDescID := messageFields[0].Descriptor()
	// message.DefaultID holds the default value on creation for the id field.
	message.DefaultID = messageDescID.Default.(func() uuid.UUID)
	snapshotFields := schema.Snapshot{}.Fields()
	_ = snapshotFields
	// snapshotDescName is the schema descriptor for name field.
	snapshotDescName := snapshotFields[2].Descriptor()
	// snapshot.NameValidator is a validator for the "name" field. It is called by the builders before save.
	snapshot.NameValidator = snapshotDescName.Validators[0].(func(string) error)
	// snapshotDescCreatedAt is the schema descriptor for createdAt field.
	snapshotDescCreatedAt := snapshotFields[3].Descriptor()
	// snapshot.DefaultCreatedAt holds the default value on creation for the createdAt field.
	snapshot.DefaultCreatedAt = snapshotDescCreatedAt.Default.(func() time.Time)
	// snapshotDescID is the schema descriptor for id field.
	snapshotDescID := snapshotFields[0].Descriptor()
	// snapshot.DefaultID holds the default value on creation for the id field.
	snapshot.DefaultID = snapshotDescID.Default.(func() uuid.UUID)
	subscriptionHooks := schema.Subscription{}.Hooks()
	subscription.Hooks[0] = subscriptionHooks[0]
	subscriptionFields := schema.Subscription{}.Fields()
	_ = subscriptionFields
	// subscriptionDescName is the schema descriptor for name field.
	subscriptionDescName := subscriptionFields[2].Descriptor()
	// subscription.NameValidator is a validator for the "name" field. It is called by the builders before save.
	subscription.NameValidator = subscriptionDescName.Validators[0].(func(string) error)
	// subscriptionDescCreatedAt is the schema descriptor for createdAt field.
	subscriptionDescCreatedAt := subscriptionFields[3].Descriptor()
	// subscription.DefaultCreatedAt holds the default value on creation for the createdAt field.
	subscription.DefaultCreatedAt = subscriptionDescCreatedAt.Default.(func() time.Time)
	// subscriptionDescLive is the schema descriptor for live field.
	subscriptionDescLive := subscriptionFields[5].Descriptor()
	// subscription.DefaultLive holds the default value on creation for the live field.
	subscription.DefaultLive = subscriptionDescLive.Default.(bool)
	// subscriptionDescOrderedDelivery is the schema descriptor for orderedDelivery field.
	subscriptionDescOrderedDelivery := subscriptionFields[9].Descriptor()
	// subscription.DefaultOrderedDelivery holds the default value on creation for the orderedDelivery field.
	subscription.DefaultOrderedDelivery = subscriptionDescOrderedDelivery.Default.(bool)
	// subscriptionDescDeliveryDelay is the schema descriptor for deliveryDelay field.
	subscriptionDescDeliveryDelay := subscriptionFields[17].Descriptor()
	// subscription.DefaultDeliveryDelay holds the default value on creation for the deliveryDelay field.
	subscription.DefaultDeliveryDelay = subscriptionDescDeliveryDelay.Default.(sqltypes.Interval)
	// subscriptionDescID is the schema descriptor for id field.
	subscriptionDescID := subscriptionFields[0].Descriptor()
	// subscription.DefaultID holds the default value on creation for the id field.
	subscription.DefaultID = subscriptionDescID.Default.(func() uuid.UUID)
	topicHooks := schema.Topic{}.Hooks()
	topic.Hooks[0] = topicHooks[0]
	topicFields := schema.Topic{}.Fields()
	_ = topicFields
	// topicDescName is the schema descriptor for name field.
	topicDescName := topicFields[1].Descriptor()
	// topic.NameValidator is a validator for the "name" field. It is called by the builders before save.
	topic.NameValidator = topicDescName.Validators[0].(func(string) error)
	// topicDescCreatedAt is the schema descriptor for createdAt field.
	topicDescCreatedAt := topicFields[2].Descriptor()
	// topic.DefaultCreatedAt holds the default value on creation for the createdAt field.
	topic.DefaultCreatedAt = topicDescCreatedAt.Default.(func() time.Time)
	// topicDescLive is the schema descriptor for live field.
	topicDescLive := topicFields[3].Descriptor()
	// topic.DefaultLive holds the default value on creation for the live field.
	topic.DefaultLive = topicDescLive.Default.(bool)
	// topicDescID is the schema descriptor for id field.
	topicDescID := topicFields[0].Descriptor()
	// topic.DefaultID holds the default value on creation for the id field.
	topic.DefaultID = topicDescID.Default.(func() uuid.UUID)
}

const (
	Version = "v0.14.4"                                         // Version of ent codegen.
	Sum     = "h1:/DhDraSLXIkBhyiVoJeSshr4ZYi7femzhj6/TckzZuI=" // Sum of ent codegen.
)
