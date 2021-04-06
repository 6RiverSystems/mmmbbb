package actions

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.6river.tech/mmmbbb/version"
)

var messageStreamerFlowControlWait = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: version.AppName,
	Name:      "message_streamer_flow_control_wait",
	Help:      "histogram of time message streamer spends waiting on flow control",
	Buckets:   extendedBuckets,
})
var messageStreamerAutoDelays = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: version.AppName,
	Name:      "message_streamer_auto_delays",
	Help:      "counter of messages automatically delayed (mod-ack equivalent) by streamer",
})
