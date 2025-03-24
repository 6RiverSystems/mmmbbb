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
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

type Interval time.Duration

// valueScanner replicates an equivalent interface in ent
type valueScanner interface {
	driver.Valuer
	sql.Scanner
}

var (
	_ driver.Valuer    = Interval(0)
	_ valueScanner     = (*Interval)(nil)
	_ json.Marshaler   = Interval(0)
	_ json.Unmarshaler = (*Interval)(nil)
)

func (i Interval) String() string {
	return time.Duration(i).String()
}

func (i Interval) Value() (driver.Value, error) {
	// PostgreSQL understands Go's duration string format
	return i.String(), nil
}

func (i *Interval) Scan(src interface{}) error {
	switch s := src.(type) {
	case time.Duration:
		*i = Interval(s)
	case *time.Duration:
		*i = Interval(*s)
	case int64:
		*i = Interval(time.Duration(s))
	case *int64:
		*i = Interval(time.Duration(*s))
	case string:
		d, err := ParsePostgreSQLInterval(s)
		if err == nil {
			*i = Interval(d)
		}
		return err
	case nil:
		// we need to be able to scan `nil` values even with ent's NullScanner in
		// place
		*i = 0
	default:
		return fmt.Errorf("unsupported scan type: %T", src)
	}
	return nil
}

func (i Interval) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

func (i *Interval) UnmarshalJSON(data []byte) error {
	var s string
	var err error
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	d, err := ParsePostgreSQLInterval(s)
	if err == nil {
		*i = Interval(d)
	}
	return err
}

func IntervalPtr(duration time.Duration) *Interval {
	ret := Interval(duration)
	return &ret
}

// parses a string in postgresql format as an interval (duration).
// Example PG format:
// `select '1 year 2 months 3 days 4 hours 5 minutes 6 seconds 7 milliseconds 8 microseconds'::interval`
// `1 year 2 mons 3 days 04:05:06.007008`
// PG only supports ns-level resolution
func ParsePostgreSQLInterval(s string) (result time.Duration, err error) {
	// TODO: using a regexp to parse this is simple, but inefficient
	matches := pgIntervalRegexp.FindStringSubmatch(s)
	if matches == nil {
		// try to parse in Go format, happens with sqlite
		result, err = time.ParseDuration(s)
		if err != nil {
			err = errors.New("unrecognized interval format")
		}
		return
	}
	// FIXME: extract the SubexpIndex values to constants
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("years")], year); err != nil {
		return
	}
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("months")], month); err != nil {
		return
	}
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("days")], day); err != nil {
		return
	}
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("hours")], time.Hour); err != nil {
		return
	}
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("minutes")], time.Minute); err != nil {
		return
	}
	if err = adjustDuration(&result, matches[pgIntervalRegexp.SubexpIndex("seconds")], time.Second); err != nil {
		return
	}
	// sub-seconds require more logic, as the scale depends on the length
	subsecs := matches[pgIntervalRegexp.SubexpIndex("subseconds")]
	if len(subsecs) != 0 {
		// PG cannot store more than microsecond resolution, but we can at least
		// tolerate up to what Go can represent on input
		if len(subsecs) > 9 {
			err = errors.New("cannot parse beyond nanosecond resolution")
			return
		}
		// len(subsecs) is in the range [1..9], so we know that
		// int64(math.Pow10(...)) will be exactly correct, and evenly divide
		// time.Second
		subsecscale := time.Second / time.Duration(math.Pow10(len(subsecs)))
		if err = adjustDuration(&result, subsecs, subsecscale); err != nil {
			return
		}
	}

	return
}

func adjustDuration(d *time.Duration, value string, scale time.Duration) error {
	if len(value) == 0 {
		return nil
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	*d += time.Duration(i) * scale
	return nil
}

var pgIntervalRegexp = regexp.MustCompile(
	`^((?P<years>[+-]?\d+) year[s]? )?` +
		`((?P<months>[+-]?\d+) mon[s]? )?` +
		`((?P<days>[+-]?\d+) day[s]? )?` +
		`(?P<hours>[+-]?\d+):(?P<minutes>[+-]?\d+):(?P<seconds>[+-]?\d+)(\.(?P<subseconds>\d+))?$`,
)

// FIXME: PG understands that years, months, and days are relative to some
// specific time, we just assume 24 hour days, 30 day months, and 365 day years

const (
	day   = 24 * time.Hour
	month = 30 * day
	year  = 365 * day
)
