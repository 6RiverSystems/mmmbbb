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

package sqltypes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParsePostgreSQLInterval(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantResult time.Duration
		assertion  assert.ErrorAssertionFunc
	}{
		{
			"hh:mm:ss",
			"01:02:03",
			1*time.Hour + 2*time.Minute + 3*time.Second,
			assert.NoError,
		},
		{
			"hh:mm:ss.s",
			"01:02:03.4",
			1*time.Hour + 2*time.Minute + 3*time.Second + 400*time.Millisecond,
			assert.NoError,
		},
		{
			"hh:mm:ss.sssssssss",
			"00:00:00.123456789",
			123456789 * time.Nanosecond,
			assert.NoError,
		},
		{
			"y m d hh:mm:ss.ssssss",
			"1 year 2 mons 3 days 08:09:10.123456",
			365*24*time.Hour +
				2*30*24*time.Hour +
				3*24*time.Hour +
				8*time.Hour +
				9*time.Minute +
				10*time.Second +
				123456*time.Microsecond,
			assert.NoError,
		},
		{
			"err hh:mm:ss.sssssssssS",
			"00:00:00.0000000009",
			time.Duration(0),
			assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := ParsePostgreSQLInterval(tt.input)
			tt.assertion(t, err)
			assert.Equal(t, tt.wantResult, gotResult)
		})
	}
}

func TestInterval_Scan(t *testing.T) {
	tests := []struct {
		name            string
		src             interface{}
		errAssertion    assert.ErrorAssertionFunc
		resultAssertion func(t *testing.T, i Interval)
	}{
		// TODO: Duration
		// TODO: *Duration
		// TODO: int64
		// TODO: *in64
		{
			"string hms",
			"1h0m0s",
			assert.NoError,
			func(t *testing.T, i Interval) {
				assert.Equal(t, time.Hour, time.Duration(i))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var i Interval
			if tt.errAssertion(t, i.Scan(tt.src)) {
				tt.resultAssertion(t, i)
			}
		})
	}
}
