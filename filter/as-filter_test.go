package filter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsFilterLoop(t *testing.T) {
	filters := []string{
		`attributes:x`,
		`attributes:x OR attributes:y`,
		`(attributes:x OR attributes:y)`,
		`(attributes:x AND attributes:y) OR attributes:z`,
		`attributes:x AND (attributes:y OR attributes:z)`,
		// looped will be compact, so we need to omit all whitespace
		`attributes.x="x"`,
		`hasPrefix(attributes.x,"x")`,
		`attributes:"foo\nbar\"yikes"`,
		`attributes:"foo bar"`,
		`hasPrefix(attributes."foo\nbar\"yikes","x")`,
		`hasPrefix(attributes."foo bar","x")`,
	}

	for _, f := range filters {
		ast := &Filter{}
		require.NoError(t, Parser.ParseString(f, f, ast))
		var b strings.Builder
		err := ast.AsFilter(&b)
		if assert.NoError(t, err) {
			assert.Equal(t, f, b.String())
		}
	}
}
