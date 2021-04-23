package faults

import (
	"sync"
	"sync/atomic"
)

type Set struct {
	mu sync.RWMutex
	// faults maps operations to fault descriptors for them
	faults     map[string][]*Description
	needsPrune int32
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

func (s *Set) Prune() {
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
			}
		}
		if d == 0 {
			delete(s.faults, o)
		} else if d != len(l) {
			s.faults[o] = l[:d]
		}
	}
}

func (s *Set) autoPrune(threshold int32) {
	need := atomic.LoadInt32(&s.needsPrune)
	// < 0 case is in case it somehow wrapped around
	if need > threshold || need < 0 {
		s.Prune()
		atomic.CompareAndSwapInt32(&s.needsPrune, need, 0)
	}
}

// Check looks for an active fault description and runs it.
func (s *Set) Check(op string, params Parameters) error {
	defer s.autoPrune(10)
	for {
		d := s.match(op, params)
		if d == nil {
			return nil
		}

		remaining := atomic.AddInt64(&d.Count, -1)
		if remaining <= 0 {
			// the description is exhausted, time to prune
			atomic.AddInt32(&s.needsPrune, 1)
			// match will have checked this, but might race with another decrementing
			// it, so we need to check again here and retry if we lost that race.
			if remaining < 0 {
				// to verify that `Test_Check_Race` is catching any errors here, and what
				// the stochastic failure rate is, comment out this line. as of last testing, it's <3%
				continue
			}
		}

		// we pass the description by value here intentionally so the fault handler
		// cannot modify it
		return d.OnFault(*d, params)
	}
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
}
