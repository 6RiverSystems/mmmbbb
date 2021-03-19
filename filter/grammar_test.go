package filter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrammarEBNF(t *testing.T) {
	p, err := NewParser()
	require.NoError(t, err)

	t.Log(p.String())
}
