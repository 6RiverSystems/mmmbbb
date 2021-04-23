package faults

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go.6river.tech/mmmbbb/version"
)

// TODO: to refactor this to gosix, we'll need a way to inject the AppName.
// May need to make the fault set part of the registry.

var faultsTriggered = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: "fault_injection",
	Name:      "triggered",
	Help:      "Number of faults triggered, by operation",
}, []string{"operation"})

var faultsAdded = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: "fault_injection",
	Name:      "added",
	Help:      "Number of fault injection descriptors added since app start",
})

var faultsExpired = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: version.AppName,
	Subsystem: "fault_injection",
	Name:      "expired",
	Help:      "Number of fault injection descriptors expired since app start",
})

// active faults has to be a custom collector because we need it to be a
// GaugeFuncVec, which doesn't exist
var activeFaultsDesc = prometheus.NewDesc(
	prometheus.BuildFQName(
		version.AppName,
		"fault_injection",
		"active",
	),
	"Number of fault injection descriptors currently active, by operation",
	[]string{"operation"},
	nil,
)
var remainingFaultsDesc = prometheus.NewDesc(
	prometheus.BuildFQName(
		version.AppName,
		"fault_injection",
		"remaining",
	),
	"Number of remaining invocations of fault injection descriptors currently remaining, by operation",
	[]string{"operation"},
	nil,
)

type ActiveFaultsCollector struct {
	set *Set
}

func NewActiveFaultsCollector(s *Set) *ActiveFaultsCollector {
	return &ActiveFaultsCollector{s}
}

func (c *ActiveFaultsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- activeFaultsDesc
	ch <- remainingFaultsDesc
}

func (c *ActiveFaultsCollector) Collect(ch chan<- prometheus.Metric) {
	// we could use Set.Current(), but we can avoid a lot of copying with custom
	// code here
	c.set.mu.RLock()
	defer c.set.mu.RUnlock()
	for op, l := range c.set.faults {
		active := 0
		remaining := int64(0)
		for _, d := range l {
			if r := atomic.LoadInt64(&d.Count); r > 0 {
				active++
				remaining += r
			}
		}
		ch <- prometheus.MustNewConstMetric(
			activeFaultsDesc,
			prometheus.GaugeValue,
			float64(active),
			op,
		)
		ch <- prometheus.MustNewConstMetric(
			remainingFaultsDesc,
			prometheus.GaugeValue,
			float64(remaining),
			op,
		)
	}
}

func init() {
	prometheus.MustRegister(NewActiveFaultsCollector(defaultSet))
}
