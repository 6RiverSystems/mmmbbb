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
	"fmt"
	"time"
)

type Time struct {
	// this hidden prop is to prevent it from being convertible to time.Date
	_ struct{}
	time.Time
}

const OASRFC3339Millis = "2006-01-02T15:04:05.000Z07:00"

func Now() Time {
	return Time{Time: time.Now().UTC().Truncate(time.Millisecond)}
}

func FromTime(value time.Time) Time {
	return Time{Time: value.UTC().Truncate(time.Millisecond)}
}

func FromTimePtr(value *time.Time) *Time {
	if value == nil {
		return nil
	}
	return &Time{Time: value.UTC().Truncate(time.Millisecond)}
}

func (t Time) MarshalJSON() ([]byte, error) {
	// TODO: pre-allocate to make it faster
	return []byte(fmt.Sprintf(`"%s"`, t.UTC().Format(OASRFC3339Millis))), nil
}

func (t Time) MarshalText() ([]byte, error) {
	// TODO: pre-allocate to make it faster
	return []byte(t.UTC().Format(OASRFC3339Millis)), nil
}

// Bind implements github.com/oapi-codegen/runtime.Binder, to prevent it from
// being treated as a legacy oapi-codgen Date object and being parsed as just
// the date.
func (t *Time) Bind(src string) error {
	tm, err := time.Parse(OASRFC3339Millis, src)
	if err != nil {
		return err
	}
	t.Time = tm
	return nil
}
