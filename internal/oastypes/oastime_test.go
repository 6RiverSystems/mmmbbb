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

package oastypes

import (
	"reflect"
	"testing"
	"time"
)

func TestTimeConversion(t *testing.T) {
	tt := reflect.TypeOf(Time{})
	type OapiCodegenDate struct{ time.Time }
	for _, bad := range []any{time.Time{}, OapiCodegenDate{}} {
		bt := reflect.TypeOf(bad)
		if tt.ConvertibleTo(bt) {
			t.Fatalf("oas.Time should not be convertible to %s", bt.Name())
		} else {
			t.Logf("oas.Time is not convertible to %s", bt.Name())
		}
	}
	if tt.Size() != reflect.TypeOf(time.Time{}).Size() {
		t.Fatalf("oas.Time should have same size as native time")
	} else {
		t.Logf("oas.Time is %d bytes, same as time.Time", tt.Size())
	}
}
