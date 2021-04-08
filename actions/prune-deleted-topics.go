package actions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

type PruneDeletedTopics struct {
	pruneAction
}

var _ Action = (*PruneDeletedTopics)(nil)

func NewPruneDeletedTopics(params PruneCommonParams) *PruneDeletedTopics {
	return &PruneDeletedTopics{
		pruneAction: *newPruneAction(params),
	}
}

var pruneDeletedTopicsCounter, pruneDeletedTopicsHistogram = pruneMetrics("deleted_topics")

func (a *PruneDeletedTopics) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneDeletedTopicsHistogram, tx)
	defer timer.Ended()

	// ent doesn't support limit on delete commands (that may be a PostgreSQL
	// extension), so have to do a query-then-delete
	topics, err := tx.Topic.Query().
		Where(
			topic.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
			// we rely on subscriptions being pruned to then allow topics to be pruned
			// TODO: ticket for HasRelationWith efficiency
			topic.Not(topic.HasSubscriptions()),
		).
		Limit(a.params.MaxDelete).
		All(ctx)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, len(topics))
	logger := logging.GetLogger("actions/prune-deleted-topics")
	for i, t := range topics {
		ids[i] = t.ID
		logger.Info().
			Str("topicName", t.Name).
			Stringer("topicID", t.ID).
			Time("deletedAt", *t.DeletedAt).
			Msg("pruning deleted topic")
	}

	numDeleted, err := tx.Topic.Delete().Where(topic.IDIn(ids...)).Exec(ctx)

	if err != nil {
		return err
	}

	// we assume any topic modify listeners already got woken up when the topic
	// was marked for deletion, and won't care about pruning

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneDeletedTopicsCounter.Add(float64(numDeleted)) })

	return nil
}
