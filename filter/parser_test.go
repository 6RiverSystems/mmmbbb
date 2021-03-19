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
		{"hasAttribute", `attributes:x`, &Filter{[]*OrTerm{{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"x", nil}, nil}, nil}}}}}},
		{"attrEqual", `attributes:x = "a"`, nil},
		{"attrEqualEscape", `attributes:x = "\n\u000a"`, &Filter{[]*OrTerm{{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"x", &OpValue{"=", "\n\n"}}, nil}, nil}}}}}},
		{"attrNotEqual", `attributes:x != "a"`, nil},
		{"hasPrefix", `hasPrefix(attributes:x, "a")`, nil},
		{
			"complex ordering",
			`
				attributes:x AND attributes:y
				OR
				attributes:a AND attributes:b
				OR (
					attributes:q = "x"
					AND (attributes:p = "z" OR hasPrefix(attributes:n, "\n"))
				)
			`,
			&Filter{[]*OrTerm{
				{[]*AndTerm{
					{false, &BasicExpression{&HasAttribute{"x", nil}, nil}, nil},
					{false, &BasicExpression{&HasAttribute{"y", nil}, nil}, nil},
				}},
				{[]*AndTerm{
					{false, &BasicExpression{&HasAttribute{"a", nil}, nil}, nil},
					{false, &BasicExpression{&HasAttribute{"b", nil}, nil}, nil},
				}},
				{[]*AndTerm{
					{Sub: &Condition{[]*OrTerm{{[]*AndTerm{
						{false, &BasicExpression{&HasAttribute{"q", &OpValue{OpEqual, "x"}}, nil}, nil},
						{Sub: &Condition{[]*OrTerm{
							{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"p", &OpValue{OpEqual, "z"}}, nil}, nil}}},
							{[]*AndTerm{{false, &BasicExpression{nil, &HasAttributePredicate{PredicateHasPrefix, "n", "\n"}}, nil}}},
						}}},
					}}}}},
				}},
			}},
		},
		{
			"NOT binding vs OR",
			`NOT attributes:x OR attributes:y`,
			&Filter{[]*OrTerm{
				{[]*AndTerm{{true, &BasicExpression{&HasAttribute{"x", nil}, nil}, nil}}},
				{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"y", nil}, nil}, nil}}},
			}},
		},
		{
			"NOT(OR)",
			`NOT(attributes:x OR attributes:y)`,
			&Filter{[]*OrTerm{{[]*AndTerm{{true, nil, &Condition{[]*OrTerm{
				{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"x", nil}, nil}, nil}}},
				{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"y", nil}, nil}, nil}}},
			}}}}}}},
		},
		{
			"quoted attribute",
			`attributes:"foo\nbar\"yikes"`,
			&Filter{[]*OrTerm{{[]*AndTerm{{false, &BasicExpression{&HasAttribute{"foo\nbar\"yikes", nil}, nil}, nil}}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast := &Filter{}
			require.NoError(t, Parser.ParseString(tt.name, tt.input, ast))
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
