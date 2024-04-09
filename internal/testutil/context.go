// Copyright (c) 2024 6 River Systems
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

package testutil

import (
	"context"
	"testing"
	"time"
)

type hasDeadline interface{ Deadline() (time.Time, bool) }

func Context(t testing.TB) context.Context {
	ctx := context.Background()
	var cancel context.CancelFunc
	if tt, ok := t.(hasDeadline); ok {
		if deadline, ok := tt.Deadline(); ok {
			ctx, cancel = context.WithDeadline(ctx, deadline)
		}
	}
	if cancel == nil {
		ctx, cancel = context.WithCancel(ctx)
	}
	t.Cleanup(cancel)
	return ctx
}

func DeadlineForTest(t testing.TB) time.Time {
	if tt, ok := t.(hasDeadline); ok {
		if deadline, ok := tt.Deadline(); ok {
			return deadline
		}
	}
	return time.Now().Add(time.Minute)
}
