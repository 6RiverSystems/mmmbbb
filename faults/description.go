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
	Operation        string
	Parameters       Parameters
	OnFault          func(Description, Parameters) error
	Count            int64
	FaultDescription string
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
