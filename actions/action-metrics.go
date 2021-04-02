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
	}, []string{outcomeLabel})
	return counters, histogram
}
