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

package filter

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateUnit(t *testing.T) {
	tests := []struct {
		name, filter string
		attrs        map[string]string
		wantMatch    bool
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			"hasAttribute/match",
			"attributes:x",
			map[string]string{"x": ""},
			true,
			assert.NoError,
		},
		{
			"hasAttribute/no-match",
			"attributes:x",
			map[string]string{"y": ""},
			false,
			assert.NoError,
		},
		{
			"attrEqual/match",
			`attributes.x="y"`,
			map[string]string{"x": "y"},
			true,
			assert.NoError,
		},
		{
			"attrEqual/no-match",
			`attributes.x="y"`,
			map[string]string{"x": "z"},
			false,
			assert.NoError,
		},
		{
			"attrNotEqual/match",
			`attributes.x!="y"`,
			map[string]string{"x": "z"},
			true,
			assert.NoError,
		},
		{
			"attrNotEqual/no-match",
			`attributes.x!="y"`,
			map[string]string{"x": "y"},
			false,
			assert.NoError,
		},
		{
			"hasPrefix/match",
			`hasPrefix(attributes.x,"y")`,
			map[string]string{"x": "yx"},
			true,
			assert.NoError,
		},
		{
			"hasPrefix/no-match",
			`hasPrefix(attributes.x,"y")`,
			map[string]string{"x": "xx"},
			false,
			assert.NoError,
		},
		{
			"precedence/NOT-OR/tf=f",
			"NOT attributes:x OR attributes:y",
			map[string]string{"x": ""},
			false,
			assert.NoError,
		},
		{
			"precedence/NOT-OR/tt=f",
			"NOT attributes:x OR attributes:y",
			map[string]string{"y": ""},
			true,
			assert.NoError,
		},
		{
			"precedence/NOT-OR/ff=t",
			"NOT attributes:x OR attributes:y",
			map[string]string{"z": ""},
			true,
			assert.NoError,
		},
		{
			"precedence/NOT-AND/tf=f",
			"NOT attributes:x AND attributes:y",
			map[string]string{"x": ""},
			false,
			assert.NoError,
		},
		{
			"precedence/NOT-AND/ff=f",
			"NOT attributes:x AND attributes:y",
			map[string]string{"z": ""},
			false,
			assert.NoError,
		},
		{
			"precedence/NOT-AND/ft=t",
			"NOT attributes:x AND attributes:y",
			map[string]string{"y": ""},
			true,
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := Parser.ParseString(tt.name, tt.filter)
			require.NoError(t, err)
			gotMatch, gotErr := ast.Evaluate(tt.attrs)
			tt.wantErr(t, gotErr)
			assert.Equal(t, tt.wantMatch, gotMatch)
		})
	}
}

func TestEvaluateComplex(t *testing.T) {
	tests := []struct {
		name, filter string
		want         func(a, b, c bool) bool
	}{
		{
			"a AND b OR c",
			"(attributes:a AND attributes:b) OR attributes:c",
			func(a, b, c bool) bool { return a && b || c },
		},
		{
			"a OR b AND c",
			"attributes:a OR (attributes:b AND attributes:c)",
			func(a, b, c bool) bool { return a || b && c },
		},
		{
			"NOT a OR b AND c",
			"NOT attributes:a OR (attributes:b AND attributes:c)",
			func(a, b, c bool) bool { return !a || b && c },
		},
		{
			"a OR NOT b AND c",
			"attributes:a OR (NOT attributes:b AND attributes:c)",
			func(a, b, c bool) bool { return a || !b && c },
		},
		{
			"a OR b AND NOT c",
			"attributes:a OR (attributes:b AND NOT attributes:c)",
			func(a, b, c bool) bool { return a || b && !c },
		},
		{
			"NOT (a OR b) AND c",
			"NOT (attributes:a OR attributes:b) AND attributes:c",
			func(a, b, c bool) bool { return !(a || b) && c },
		},
	}
	booleans := []bool{false, true}
	for _, tt := range tests {
		for _, a := range booleans {
			for _, b := range booleans {
				for _, c := range booleans {
					attrs := make(map[string]string)
					if a {
						attrs["a"] = ""
					}
					if b {
						attrs["b"] = ""
					}
					if c {
						attrs["c"] = ""
					}
					want := tt.want(a, b, c)
					t.Run(
						tt.name+"/"+shortBool(a)+shortBool(b)+shortBool(c)+"="+shortBool(want),
						func(t *testing.T) {
							ast, err := Parser.ParseString(tt.name, tt.filter)
							require.NoError(t, err)
							got, err := ast.Evaluate(attrs)
							if assert.NoError(t, err) {
								assert.Equal(t, want, got)
							}
						},
					)
				}
			}
		}
	}
}

func shortBool(b bool) string {
	return strconv.FormatBool(b)[:1]
}
