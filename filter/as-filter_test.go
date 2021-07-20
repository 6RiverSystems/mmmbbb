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
