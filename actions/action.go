// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package actions

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"go.6river.tech/mmmbbb/ent"
)

type Action[P any, R any] interface {
	ActionData[P, R]
	Execute(context.Context, *ent.Tx) error
}
type ActionData[P any, R any] interface {
	Parameters() P
	Results() (results R, ok bool)
}

type actionBase[P, R any] struct {
	params  P
	results *R
}

var _ ActionData[string, int] = (*actionBase[string, int])(nil)

func (a *actionBase[P, R]) Parameters() P {
	return a.params
}
func (a *actionBase[P, R]) Results() (results R, ok bool) {
	if a.results != nil {
		results, ok = *a.results, true
	}
	return
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
