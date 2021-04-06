package actions

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.6river.tech/mmmbbb/version"
)

var httpPushFailures = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: version.AppName,
		Name:      "http_push_streamer_failures",
		Help:      "Number of attempted HTTP pushes that failed",
	},
	[]string{"status"},
)
var httpPushSuccesses = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: version.AppName,
		Name:      "http_push_streamer_successes",
		Help:      "Number of attempted HTTP pushes that failed",
	},
	[]string{"status"},
)
