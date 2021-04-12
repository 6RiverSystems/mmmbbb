package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/version"
)

const (
	directionOutbound = "outbound"
	directionInbound  = "inbound"
)

const mirrorSubsystem = "topic_mirror"

var mirrorLabels = []string{"direction"}

var mirroredTopicsGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "num_topics",
}, mirrorLabels)

var createdTopicsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "created_topics",
}, mirrorLabels)

var watchersStartedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "watchers_started",
}, mirrorLabels)
var watchersEndedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "watchers_ended",
}, mirrorLabels)

var messagesMirroredCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "messages_mirrored",
}, mirrorLabels)

const (
	dropReasonNotJSON = "not_json"
	dropReasonLoop    = "loop"
)

var messagesDroppedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: mirrorSubsystem,
	Name:      "messages_dropped",
}, append(mirrorLabels, "reason"))
