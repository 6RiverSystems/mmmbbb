package actions

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/version"
)

var getSubscriptionMessagesCounter, getSubscriptionMessagesHistogram = actionMetrics("get_subscription_messages", "messages", "delivered")

var getSubscriptionMessagesLockTime = promauto.NewHistogram(prometheus.HistogramOpts{
	Namespace: version.AppName,
	Name:      "get_subscription_messages_lock_wait_time",
	Help:      "time spent by GetSubscriptionMessages waiting for a DB advisory lock on a subscription",
	Buckets:   extendedBuckets,
})
