package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/ent"
)

type Action interface {
	Execute(context.Context, *ent.Tx) error

	// individual types should declare accessors for specific results

	// these are for diag/logging, so strong typing isn't critical

	Parameters() map[string]interface{}
	HasResults() bool
	Results() map[string]interface{}
}

const outcomeLabel = "outcome"

// the action succeeded and was committed to the db
const outcomeSuccess = "success"

// the action failed and was rolled back because of that
const outcomeFailure = "failure"

// the action ran to completion, but commit failed, and so it was rolled back
const outcomeCanceled = "canceled"

func actionMetrics(name, subject, verb string) (prometheus.Counter, *prometheus.HistogramVec) {
	counters, histogram := actionMetricsMulti(name, []struct {
		subject string
		verb    string
	}{{subject, verb}})
	return counters[0], histogram
}

func actionMetricsMulti(
	name string,
	items []struct{ subject, verb string },
) ([]prometheus.Counter, *prometheus.HistogramVec) {
	prettyName := strings.ReplaceAll(name, "_", " ")
	counters := make([]prometheus.Counter, len(items))
	for i, sv := range items {
		counters[i] = promauto.NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_num_%s", name, sv.verb),
			Help: fmt.Sprintf("Number of %s %s in calls to %s", sv.subject, sv.verb, prettyName),
		})
	}
	histogram := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: fmt.Sprintf("%s_duration", name),
		Help: fmt.Sprintf("Runtime histogram for %s, by outcome", prettyName),
	}, []string{outcomeLabel})
	return counters, histogram
}

// actionTimer is a variation on a prometheus.Timer, where it records the
// duration at a separate time from observing it, so that the labels can be
// determined e.g. by whether the tx commit succeeds.
type actionTimer struct {
	begin, end time.Time
	metric     prometheus.ObserverVec
	succeeded  bool
	reported   bool
	onSuccess  func()
}

func startActionTimer(metric prometheus.ObserverVec, tx *ent.Tx) *actionTimer {
	timer := &actionTimer{
		begin:  time.Now(),
		metric: metric,
	}

	if tx != nil {
		tx.OnRollback(func(r ent.Rollbacker) ent.Rollbacker {
			return ent.RollbackFunc(func(ctx context.Context, t *ent.Tx) error {
				timer.ReportRollback()
				return r.Rollback(ctx, t)
			})
		})
		tx.OnCommit(func(c ent.Committer) ent.Committer {
			return ent.CommitFunc(func(ctx context.Context, t *ent.Tx) error {
				err := c.Commit(ctx, t)
				if err != nil {
					timer.ReportRollback()
				} else {
					timer.ReportCommit()
				}
				return err
			})
		})
	}

	return timer
}

func (at *actionTimer) Succeeded(onSuccess func()) {
	at.end = time.Now()
	at.succeeded = true
	at.onSuccess = onSuccess
}

func (at *actionTimer) Failed() {
	at.end = time.Now()
	at.succeeded = false
}

func (at *actionTimer) Ended() {
	if at.end == (time.Time{}) {
		at.end = time.Now()
	}
}

func (at *actionTimer) ReportRollback() {
	if at.reported {
		return
	}
	if at.end == (time.Time{}) {
		at.end = time.Now()
	}
	outcome := outcomeFailure
	if at.succeeded {
		// rollback after success is canceled, not failed or succeeded
		outcome = outcomeCanceled
	}
	at.metric.
		With(prometheus.Labels{outcomeLabel: outcome}).
		Observe(at.end.Sub(at.begin).Seconds())
	at.reported = true
}

func (at *actionTimer) ReportCommit() {
	if at.reported {
		return
	}
	if at.end == (time.Time{}) {
		at.end = time.Now()
	}
	outcome := outcomeFailure
	if at.succeeded {
		outcome = outcomeSuccess
		if at.onSuccess != nil {
			at.onSuccess()
		}
	}
	at.metric.
		With(prometheus.Labels{outcomeLabel: outcome}).
		Observe(at.end.Sub(at.begin).Seconds())
	at.reported = true
}
