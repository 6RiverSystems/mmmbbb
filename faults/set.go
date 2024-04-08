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

package faults

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

type Set struct {
	mu sync.RWMutex
	// faults maps operations to fault descriptors for them
	faults map[string][]*Description

	faultsTriggered       *prometheus.CounterVec
	faultsAdded           prometheus.Counter
	faultsExpired         prometheus.Counter
	activeFaultsDesc      *prometheus.Desc
	remainingFaultsDesc   *prometheus.Desc
	activeFaultsCollector *ActiveFaultsCollector
}

func NewSet(appName string) *Set {
	// sanitize appName for prometheus
	appName = strings.ReplaceAll(appName, "-", "_")
	appName = strings.ReplaceAll(appName, "/", "_")

	ret := &Set{
		mu:     sync.RWMutex{},
		faults: map[string][]*Description{},

		faultsTriggered: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: appName,
			Subsystem: "fault_injection",
			Name:      "triggered",
			Help:      "Number of faults triggered, by operation",
		}, []string{"operation"}),

		faultsAdded: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: appName,
			Subsystem: "fault_injection",
			Name:      "added",
			Help:      "Number of fault injection descriptors added since app start",
		}),
		faultsExpired: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: appName,
			Subsystem: "fault_injection",
			Name:      "expired",
			Help:      "Number of fault injection descriptors expired since app start",
		}),

		// active faults has to be a custom collector because we need it to be a
		// GaugeFuncVec, which doesn't exist
		activeFaultsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(
				appName,
				"fault_injection",
				"active",
			),
			"Number of fault injection descriptors currently active, by operation",
			[]string{"operation"},
			nil,
		),
		remainingFaultsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(
				appName,
				"fault_injection",
				"remaining",
			),
			"Number of remaining invocations of fault injection descriptors currently remaining, by operation",
			[]string{"operation"},
			nil,
		),
	}
	ret.activeFaultsCollector = NewActiveFaultsCollector(ret)
	return ret
}

func (s *Set) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		s.faultsTriggered,
		s.faultsAdded,
		s.faultsExpired,
		s.activeFaultsCollector,
	}
}

func (s *Set) Register(registerer prometheus.Registerer) error {
	for _, c := range s.collectors() {
		if err := registerer.Register(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Set) MustRegister(registerer prometheus.Registerer) {
	for _, c := range s.collectors() {
		registerer.MustRegister(c)
	}
}

func (s *Set) match(op string, params Parameters) *Description {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dl := s.faults[op]
	for _, d := range dl {
		if d.match(op, params) {
			return d
		}
	}
	return nil
}

func (s *Set) prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for o, l := range s.faults {
		dest := 0
		for src := range l {
			if atomic.LoadInt64(&l[src].Count) > 0 {
				if src != dest {
					l[dest] = l[src]
				}
				dest++
			} else {
				s.faultsExpired.Inc()
			}
		}
		if dest == 0 {
			delete(s.faults, o)
		} else if dest != len(l) {
			s.faults[o] = l[:dest]
		}
	}
}

// Check looks for an active fault description and runs it.
func (s *Set) Check(op string, params Parameters) error {
	prune := false
	var ret error
	for {
		d := s.match(op, params)
		if d == nil {
			return nil
		}

		remaining := atomic.AddInt64(&d.Count, -1)
		if remaining <= 0 {
			prune = true
			// match will have checked this, but might race with another decrementing
			// it, so we need to check again here and retry if we lost that race.
			if remaining < 0 {
				// to verify that `Test_Check_Race` is catching any errors here, and what
				// the stochastic failure rate is, comment out this line. as of last testing, it's <3%
				continue
			}
		}

		s.faultsTriggered.WithLabelValues(d.Operation).Inc()

		// careful copying to avoid data race errors, don't need to copy the fault
		// func we're about to call
		dd := Description{d.Operation, d.Parameters, nil, remaining, d.FaultDescription}
		// we pass the description by value here intentionally so the fault handler
		// cannot modify it
		ret = d.OnFault(dd, params)
		break
	}
	if prune {
		// don't wait for the prune
		go s.prune()
	}
	return ret
}

func (s *Set) Add(d Description) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.faults[d.Operation]
	if len(l) == 0 {
		// avoid keeping high capacity arrays around when not needed
		s.faults[d.Operation] = []*Description{&d}
	} else {
		s.faults[d.Operation] = append(l, &d)
	}
	s.faultsAdded.Inc()
}

// Current gets a copy of the currently configured set of faults. Due to
// concurrency considerations, the remaining count on the descriptions may be
// out of date by the time the value is returned.
func (s *Set) Current() map[string][]Description {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make(map[string][]Description, len(s.faults))
	for o, l := range s.faults {
		ll := make([]Description, 0, len(l))
		for _, d := range l {
			// make a copy before we look at it
			dd := Description{d.Operation, d.Parameters, d.OnFault, atomic.LoadInt64(&d.Count), d.FaultDescription}
			if dd.Count > 0 {
				ll = append(ll, dd)
			}
		}
		if len(ll) != 0 {
			ret[o] = ll
		}
	}
	return ret
}
