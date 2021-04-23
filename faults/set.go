package faults

import (
	"sync"
	"sync/atomic"
)

type Set struct {
	mu sync.RWMutex
	// faults maps operations to fault descriptors for them
	faults map[string][]*Description
}

func NewSet() *Set {
	return &Set{
		sync.RWMutex{},
		map[string][]*Description{},
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
		var d = 0
		for s := range l {
			if atomic.LoadInt64(&l[s].Count) > 0 {
				if s != d {
					l[d] = l[s]
				}
				d++
			} else {
				faultsExpired.Inc()
			}
		}
		if d == 0 {
			delete(s.faults, o)
		} else if d != len(l) {
			s.faults[o] = l[:d]
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

		faultsTriggered.WithLabelValues(d.Operation).Inc()

		// careful copying to avoid data race errors, don't need to copy the fault
		// func we're about to call
		dd := Description{d.Operation, d.Parameters, nil, remaining}
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
	faultsAdded.Inc()
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
			dd := Description{d.Operation, d.Parameters, d.OnFault, atomic.LoadInt64(&d.Count)}
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
