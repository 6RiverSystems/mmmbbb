package actions

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.6river.tech/mmmbbb/version"
)

var publishedMessagesCounter, enqueuedDeliveriesCounter prometheus.Counter
var publishedMessagesHistogram *prometheus.HistogramVec

var filterErrorsCounter, filterNoMatchCounter prometheus.Counter

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
}
