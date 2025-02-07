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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserBinding(t *testing.T) {
	tests := []struct {
		name, input string
		want        *Filter
	}{
		{"hasAttribute", `attributes:x`, &Filter{Term: &Term{Basic: &BasicExpression{Has: &HasAttribute{"x"}}}}},
		{"attrEqual", `attributes.x = "a"`, nil},
		{
			"attrEqualEscape",
			`attributes.x = "\n\u000a"`,
			&Filter{Term: &Term{Basic: &BasicExpression{Value: &HasAttributeValue{"x", OpEqual, "\n\n"}}}},
		},
		{"attrNotEqual", `attributes.x != "a"`, nil},
		{"hasPrefix", `hasPrefix(attributes.x, "a")`, nil},
		{
			"complex ordering",
			`
				(attributes:x AND attributes:y)
				OR
				(attributes:a AND attributes:b)
				OR (
					attributes.q = "x"
					AND (attributes.p = "z" OR hasPrefix(attributes.n, "\n"))
				)
			`,
			&Filter{
				Term: &Term{Sub: &Condition{
					Term: &Term{Basic: &BasicExpression{Has: &HasAttribute{"x"}}},
					And: []*Term{
						{Basic: &BasicExpression{Has: &HasAttribute{"y"}}},
					},
				}},
				Or: []*Term{
					{Sub: &Condition{
						Term: &Term{Basic: &BasicExpression{Has: &HasAttribute{"a"}}},
						And: []*Term{
							{Basic: &BasicExpression{Has: &HasAttribute{"b"}}},
						},
					}},
					{Sub: &Condition{
						Term: &Term{Basic: &BasicExpression{Value: &HasAttributeValue{"q", OpEqual, "x"}}},
						And: []*Term{{
							Sub: &Condition{
								Term: &Term{Basic: &BasicExpression{Value: &HasAttributeValue{"p", OpEqual, "z"}}},
								Or: []*Term{
									{Basic: &BasicExpression{Predicate: &HasAttributePredicate{PredicateHasPrefix, "n", "\n"}}},
								},
							},
						}},
					}},
				},
			},
		},
		{
			"NOT binding vs OR",
			`NOT attributes:x OR attributes:y`,
			&Filter{
				Term: &Term{Not: true, Basic: &BasicExpression{Has: &HasAttribute{"x"}}},
				Or: []*Term{
					{Basic: &BasicExpression{Has: &HasAttribute{"y"}}},
				},
			},
		},
		{
			"NOT(OR)",
			`NOT(attributes:x OR attributes:y)`,
			&Filter{Term: &Term{
				Not: true,
				Sub: &Condition{
					Term: &Term{Basic: &BasicExpression{Has: &HasAttribute{"x"}}},
					Or:   []*Term{{Basic: &BasicExpression{Has: &HasAttribute{"y"}}}},
				},
			}},
		},
		{
			"quoted attribute",
			`attributes:"foo\nbar\"yikes"`,
			&Filter{Term: &Term{Basic: &BasicExpression{Has: &HasAttribute{"foo\nbar\"yikes"}}}},
		},
		{
			"unary not",
			`-attributes:x`,
			&Filter{Term: &Term{Not: true, Basic: &BasicExpression{Has: &HasAttribute{"x"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := Parser.ParseString(tt.name, tt.input)
			require.NoError(t, err)
			// t.Logf("%#v", ast)
			j, err := json.MarshalIndent(&ast, "", " ")
			require.NoError(t, err)
			if tt.want != nil {
				if !assert.Equal(t, tt.want, ast) {
					t.Log(string(j))
				}
			} else {
				assert.NotEmpty(t, j)
				// t.Log(string(j))
			}
		})
	}
}
