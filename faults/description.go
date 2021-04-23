package faults

import "sync/atomic"

type Parameters = map[string]string

// Description defines a configured fault to inject. Faults are matched on an
// Operation and Parameters. To match, the Operation must be exactly the same,
// and all Parameters entries present in the Description must be present in the
// actual operation (but not vice versa!). A configured fault will execute at
// most Count times before being disabled. When it does execute, OnFault will be
// executed, and any error it returns will be returned to the caller. At most
// one Description is executed for a given invocation. If multiple match, which
// runs is undefined.
type Description struct {
	Operation  string
	Parameters Parameters
	OnFault    func(Description, Parameters) error
	Count      int64
}

func (d *Description) match(op string, params Parameters) bool {
	if atomic.LoadInt64(&d.Count) <= 0 {
		return false
	}
	if d.Operation != op {
		return false
	}
	for p, v := range d.Parameters {
		if vv, ok := params[p]; !ok {
			return false
		} else if vv != v {
			return false
		}
	}
	return true
}
