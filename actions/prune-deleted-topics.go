package actions

import (
	"context"
	"time"

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

	var del *ent.TopicDelete
	cond := topic.And(
		topic.DeletedAtLTE(time.Now().Add(-a.params.MinAge)),
		// we rely on subscriptions being pruned to then allow topics to be pruned
		// TODO: ticket for HasRelationWith efficiency
		topic.Not(topic.HasSubscriptions()),
	)
	if a.params.MaxDelete == 0 {
		del = tx.Topic.Delete().Where(cond)
	} else {
		// ent doesn't support limit on delete commands (that may be a PostgreSQL
		// extension), so have to do a query-then-delete
		ids, err := tx.Topic.Query().Where(cond).Limit(a.params.MaxDelete).IDs(ctx)
		if err != nil {
			return err
		}
		del = tx.Topic.Delete().Where(topic.IDIn(ids...))
	}

	numDeleted, err := del.Exec(ctx)
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
