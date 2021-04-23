package faults

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func desc(op string, count int64) *Description {
	return &Description{Operation: op, Count: count}
}

func TestSet_match(t *testing.T) {
	type args struct {
		op     string
		params Parameters
	}
	tests := []struct {
		name   string
		faults map[string][]*Description
		args   args
		want   *Description
	}{
		{
			"nil",
			nil,
			args{"op", nil},
			nil,
		},
		{
			"empty",
			map[string][]*Description{},
			args{"op", nil},
			nil,
		},
		{
			"wrong op",
			map[string][]*Description{
				"op1": {desc("op1", 1)},
			},
			args{"op2", nil},
			nil,
		},
		{
			"right op",
			map[string][]*Description{
				"op1": {desc("op1", 1)},
			},
			args{"op1", nil},
			desc("op1", 1),
		},
		{
			// this checks it doesn't waste time on the wrong path by making it panic
			// if it did
			"only check right op",
			map[string][]*Description{
				"op1": {desc("op1", 1)},
				"op2": {nil},
			},
			args{"op1", nil},
			desc("op1", 1),
		},
		{
			"first match",
			map[string][]*Description{
				"op": {desc("op", 1), desc("op", 2)},
			},
			args{"op", nil},
			desc("op", 1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Set{
				mu:     sync.RWMutex{},
				faults: tt.faults,
			}
			assert.Equal(t, tt.want, s.match(tt.args.op, tt.args.params))
		})
	}
}

type faultWrap struct {
	desc   Description
	params Parameters
}

// implement error
func (f *faultWrap) Error() string {
	return "fault wrapper"
}

func TestSet_Check(t *testing.T) {
	type args struct {
		op     string
		params Parameters
	}
	tests := []struct {
		name      string
		faults    map[string][]*Description
		args      args
		assertion assert.ErrorAssertionFunc
		post      func(assert.TestingT, *Set)
	}{
		{
			"decrement count",
			map[string][]*Description{
				"op": {&Description{"op", nil, func(desc Description, params Parameters) error {
					return &faultWrap{desc, params}
				}, 1}},
			},
			args{"op", nil},
			func(tt assert.TestingT, e error, i ...interface{}) bool {
				if !assert.IsType(tt, &faultWrap{}, e) {
					return false
				}
				fw := e.(*faultWrap)
				return assert.Equal(tt, int64(0), fw.desc.Count)
			},
			func(tt assert.TestingT, s *Set) {
				d := s.faults["op"][0]
				assert.Equal(tt, int64(0), d.Count)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Set{
				mu:     sync.RWMutex{},
				faults: tt.faults,
			}
			// holding an RLock here prevents the automatic Prune from interfering
			s.mu.RLock()
			defer s.mu.RUnlock()
			e := s.Check(tt.args.op, tt.args.params)
			if tt.assertion(t, e) {
				tt.post(t, s)
			}
		})
	}
}

func TestSet_Check_Race(t *testing.T) {
	// testing that raced calls to Check deal with overlapping decrements is
	// stochastic, so we need to repeat it a bunch to have confidence. 1000
	// attempts with 10 concurrent accesses reliably gets ~1-3% of attempts
	// failing.
	attempts := 1000
	concurrent := 10
	if testing.Short() {
		attempts /= 10
	}
	fails := 0

	for n := 0; n < attempts; n++ {
		c := int64(0)
		onFault := func(Description, Parameters) error {
			atomic.AddInt64(&c, 1)
			return nil
		}
		s := &Set{
			sync.RWMutex{},
			map[string][]*Description{
				"op": {&Description{"op", nil, onFault, 1}},
			},
		}
		wg := &sync.WaitGroup{}
		wg.Add(concurrent)
		// use a wee spinlock to get all the goroutines to wake up as close to the
		// same time as possible
		spin := int32(1)
		for i := 0; i < concurrent; i++ {
			go func() {
				for atomic.LoadInt32(&spin) != 0 {
				}
				assert.Nil(t, s.Check("op", nil))
				wg.Done()
			}()
		}
		atomic.StoreInt32(&spin, 0)
		wg.Wait()
		// if !assert.Equal(t, int64(1), atomic.LoadInt64(&c)) {
		if atomic.LoadInt64(&c) != 1 {
			fails++
		}
	}

	// these just make it easier to work out the pass/fail rate from the output
	assert.Equal(t, 0, fails, "should fail zero attempts")
	assert.Equal(t, attempts, attempts-fails, "should pass all attempts")
}

func TestSet_Prune(t *testing.T) {
	tests := []struct {
		name   string
		faults map[string][]*Description
		want   map[string][]*Description
	}{
		{
			"nil",
			nil,
			nil,
		},
		{
			"empty",
			map[string][]*Description{},
			map[string][]*Description{},
		},
		{
			"expired to empty",
			map[string][]*Description{
				"op": {desc("op", 0)},
			},
			map[string][]*Description{},
		},
		{
			"keep before expired",
			map[string][]*Description{
				"op": {desc("op", 1), desc("op", 0)},
			},
			map[string][]*Description{
				"op": {desc("op", 1)},
			},
		},
		{
			"keep after expired",
			map[string][]*Description{
				"op": {desc("op", 0), desc("op", 1)},
			},
			map[string][]*Description{
				"op": {desc("op", 1)},
			},
		},
		{
			"keep interleaved expired",
			map[string][]*Description{
				"op": {desc("op", 0), desc("op", 1), desc("op", 0), desc("op", 2)},
			},
			map[string][]*Description{
				"op": {desc("op", 1), desc("op", 2)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Set{
				mu:     sync.RWMutex{},
				faults: tt.faults,
			}
			s.prune()
			assert.Equal(t, tt.want, s.faults)
		})
	}
}

func TestSet_Add(t *testing.T) {
	tests := []struct {
		name   string
		faults map[string][]*Description
		add    Description
		want   map[string][]*Description
	}{
		// add to nil isn't expected tot work
		{
			"add to empty",
			map[string][]*Description{},
			*desc("op", 1),
			map[string][]*Description{
				"op": {desc("op", 1)},
			},
		},
		{
			"append to existing",
			map[string][]*Description{
				"op": {desc("op", 1)},
			},
			*desc("op", 2),
			map[string][]*Description{
				"op": {desc("op", 1), desc("op", 2)},
			},
		},
		{
			"add new op",
			map[string][]*Description{
				"op1": {desc("op1", 1)},
			},
			*desc("op2", 1),
			map[string][]*Description{
				"op1": {desc("op1", 1)},
				"op2": {desc("op2", 1)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Set{
				mu:     sync.RWMutex{},
				faults: tt.faults,
			}
			s.Add(tt.add)
			assert.Equal(t, tt.want, s.faults)
		})
	}
}
