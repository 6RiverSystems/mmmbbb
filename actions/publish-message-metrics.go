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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/version"
)

var (
	publishedMessagesCounter, enqueuedDeliveriesCounter prometheus.Counter
	publishedMessagesHistogram                          *prometheus.HistogramVec
)

var filterErrorsCounter, filterNoMatchCounter prometheus.Counter

var deadLetterDeliveriesCounter prometheus.Counter

func init() {
	var counters []prometheus.Counter
	counters, publishedMessagesHistogram = actionMetricsMulti(
		"published_messages",
		[]struct {
			subject string
			verb    string
		}{
			{"messages", "published"},
			{"deliveries", "enqueued"},
		},
	)
	publishedMessagesCounter = counters[0]
	enqueuedDeliveriesCounter = counters[1]

	deliverySuppressed := promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: version.AppName,
			Name:      "delivery_suppressed",
			Help:      "Number of message deliveries suppressed",
		},
		[]string{"reason"},
	)
	filterErrorsCounter = deliverySuppressed.WithLabelValues("filter_error")
	filterNoMatchCounter = deliverySuppressed.WithLabelValues("filter_skip")

	deadLetterDeliveriesCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: version.AppName,
		Name:      "deadletter_deliveries",
		Help:      "Number of message deliveries redirected to dead letter topic",
	})
}
