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
			`attributes:x="y"`,
			map[string]string{"x": "y"},
			true,
			assert.NoError,
		},
		{
			"attrEqual/no-match",
			`attributes:x="y"`,
			map[string]string{"x": "z"},
			false,
			assert.NoError,
		},
		{
			"attrNotEqual/match",
			`attributes:x!="y"`,
			map[string]string{"x": "z"},
			true,
			assert.NoError,
		},
		{
			"attrNotEqual/no-match",
			`attributes:x!="y"`,
			map[string]string{"x": "y"},
			false,
			assert.NoError,
		},
		{
			"hasPrefix/match",
			`hasPrefix(attributes:x,"y")`,
			map[string]string{"x": "yx"},
			true,
			assert.NoError,
		},
		{
			"hasPrefix/no-match",
			`hasPrefix(attributes:x,"y")`,
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
			p, err := NewParser()
			require.NoError(t, err)
			ast := &Filter{}
			require.NoError(t, p.ParseString(tt.name, tt.filter, ast))
			gotMatch, gotErr := ast.Evaluate(tt.attrs)
			assert.Equal(t, tt.wantMatch, gotMatch)
			tt.wantErr(t, gotErr)
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
			"attributes:a AND attributes:b OR attributes:c",
			func(a, b, c bool) bool { return a && b || c },
		},
		{
			"a OR b AND c",
			"attributes:a OR attributes:b AND attributes:c",
			func(a, b, c bool) bool { return a || b && c },
		},
		{
			"NOT a OR b AND c",
			"NOT attributes:a OR attributes:b AND attributes:c",
			func(a, b, c bool) bool { return !a || b && c },
		},
		{
			"a OR NOT b AND c",
			"attributes:a OR NOT attributes:b AND attributes:c",
			func(a, b, c bool) bool { return a || !b && c },
		},
		{
			"a OR b AND NOT c",
			"attributes:a OR attributes:b AND NOT attributes:c",
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
							p, err := NewParser()
							require.NoError(t, err)
							ast := &Filter{}
							require.NoError(t, p.ParseString(tt.name, tt.filter, ast))
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
