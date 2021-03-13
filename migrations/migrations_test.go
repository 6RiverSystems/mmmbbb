package migrations

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbed(t *testing.T) {
	entries, err := MessageBusMigrations.ReadDir(".")
	assert.NoError(t, err)
	assert.NotNil(t, entries)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].IsDir())
	assert.Equal(t, entries[0].Name(), "message-bus")

	entries, err = MessageBusMigrations.ReadDir("message-bus")
	assert.NoError(t, err)
	assert.NotNil(t, entries)
	require.NotEmpty(t, entries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	assert.Equal(t, entries[0].Name(), "0001_base.up.sql")
}
