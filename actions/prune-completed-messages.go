package actions

import (
	"context"
	"time"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/message"
)

type PruneCompletedMessages struct {
	pruneAction
}

var _ Action = (*PruneCompletedMessages)(nil)

func NewPruneCompletedMessages(params PruneCommonParams) *PruneCompletedMessages {
	return &PruneCompletedMessages{
		pruneAction: *newPruneAction(params),
	}
}

var pruneCompletedMessagesCounter, pruneCompletedMessagesHistogram = pruneMetrics("completed_messages")

func (a *PruneCompletedMessages) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(pruneCompletedMessagesHistogram, tx)
	defer timer.Ended()

	var del *ent.MessageDelete
	cond := message.And(
		// messages can only be pruned by publish time, not completion time, because
		// we only get here when all the completions have been pruned
		message.PublishedAtLTE(time.Now().Add(-a.params.MinAge)),
		// we rely on deliveries being pruned to then allow messages to be pruned
		// TODO: ticket for HasRelationWith efficiency
		message.Not(message.HasDeliveries()),
	)
	if a.params.MaxDelete == 0 {
		del = tx.Message.Delete().Where(cond)
	} else {
		// ent doesn't support limit on delete commands (that may be a PostgreSQL
		// extension), so have to do a query-then-delete
		ids, err := tx.Message.Query().Where(cond).Limit(a.params.MaxDelete).IDs(ctx)
		if err != nil {
			return err
		}
		del = tx.Message.Delete().Where(message.IDIn(ids...))
	}

	numDeleted, err := del.Exec(ctx)
	if err != nil {
		return err
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}
	timer.Succeeded(func() { pruneCompletedMessagesCounter.Add(float64(numDeleted)) })

	return nil
}
