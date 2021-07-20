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
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/version"
)

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
			Namespace: version.AppName,
			Name:      fmt.Sprintf("%s_num_%s", name, sv.verb),
			Help:      fmt.Sprintf("Number of %s %s in calls to %s", sv.subject, sv.verb, prettyName),
		})
	}
	histogram := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: version.AppName,
		Name:      fmt.Sprintf("%s_duration", name),
		Help:      fmt.Sprintf("Runtime histogram for %s, by outcome", prettyName),
		Buckets:   extendedBuckets,
	}, []string{outcomeLabel})
	return counters, histogram
}

// need to initialize this with a function so that init order works correctly
var extendedBuckets = makeExtendedBuckets()

func makeExtendedBuckets() []float64 {
	ret := make([]float64, 0, len(prometheus.DefBuckets)+5)
	ret = append(ret, prometheus.DefBuckets...)
	ret = append(ret, 30, 60, 120, 300, 600)
	return ret
}
