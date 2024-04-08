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
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// TODO: to refactor this to gosix, we'll need a way to inject the AppName.
// May need to make the fault set part of the registry.

type ActiveFaultsCollector struct {
	set *Set
}

func NewActiveFaultsCollector(s *Set) *ActiveFaultsCollector {
	return &ActiveFaultsCollector{s}
}

func (c *ActiveFaultsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.set.activeFaultsDesc
	ch <- c.set.remainingFaultsDesc
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
			c.set.activeFaultsDesc,
			prometheus.GaugeValue,
			float64(active),
			op,
		)
		ch <- prometheus.MustNewConstMetric(
			c.set.remainingFaultsDesc,
			prometheus.GaugeValue,
			float64(remaining),
			op,
		)
	}
}
