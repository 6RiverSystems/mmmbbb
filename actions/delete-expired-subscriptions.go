package actions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
)

type DeleteExpiredSubscriptions struct {
	pruneAction
}

var _ Action = (*DeleteExpiredSubscriptions)(nil)

func NewDeleteExpiredSubscriptions(params PruneCommonParams) *DeleteExpiredSubscriptions {
	// this is stricter than the base validation
	if params.MaxDelete <= 0 {
		panic(errors.New("MaxDelete must be > 0"))
	}
	return &DeleteExpiredSubscriptions{
		pruneAction: *newPruneAction(params),
	}
}

var DeleteExpiredSubscriptionsCounter, DeleteExpiredSubscriptionsHistogram = pruneMetrics("expired_subscriptions")

func (a *DeleteExpiredSubscriptions) Execute(ctx context.Context, tx *ent.Tx) error {
	timer := startActionTimer(DeleteExpiredSubscriptionsHistogram, tx)
	defer timer.Ended()

	now := time.Now()
	// NOTE: notification requires knowing the subs affected, so we require a
	// specific MaxDelete for now. if ent could do limit on delete, plus returning
	// clauses, then we could do this without the pre-query
	subs, err := tx.Subscription.Query().
		Where(
			subscription.ExpiresAtLT(now),
			subscription.DeletedAtIsNil(),
			// NOTE: we don't use MinAge here: we could use it as a grace period, but
			// that doesn't seem helpful
		).
		Limit(a.params.MaxDelete).
		All(ctx)
	if err != nil {
		return err
	}

	ids := make([]uuid.UUID, len(subs))
	for i, s := range subs {
		ids[i] = s.ID
	}
	numDeleted, err := tx.Subscription.Update().
		Where(subscription.IDIn(ids...)).
		SetDeletedAt(now).
		ClearLive().
		Save(ctx)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		NotifyModifySubscription(tx, sub.ID, sub.Name)
	}

	a.results = &PruneCommonResults{
		NumDeleted: numDeleted,
	}

	timer.Succeeded(func() { DeleteExpiredSubscriptionsCounter.Add(float64(numDeleted)) })

	return nil
}
