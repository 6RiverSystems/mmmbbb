package actions

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func TestCreateTopic(t *testing.T) {
	ctx := testutils.ContextForTest(t)
	client := enttest.ClientForTest(t)

	tests := []struct {
		name    string
		before  func(t *testing.T)
		params  CreateTopicParams
		checker func(t *testing.T, err error, a *CreateTopic)
	}{
		{
			"simple create",
			nil,
			CreateTopicParams{
				Name: "xyzzy",
			},
			func(t *testing.T, err error, a *CreateTopic) {
				require.NoError(t, err)
				require.True(t, a.HasResults())
				assert.NotEqual(t, a.TopicID(), uuid.UUID{}, "Must not save with zero uuid")
			},
		},
		{
			"create with labels",
			nil,
			CreateTopicParams{
				Name: "xyzzy",
				Labels: map[string]string{
					"l1": "v1",
				},
			},
			func(t *testing.T, err error, a *CreateTopic) {
				require.NoError(t, err)
				require.True(t, a.HasResults())
				require.NotEqual(t, a.TopicID(), uuid.UUID{}, "Must not save with zero uuid")
				topic, err := client.Topic.Get(ctx, a.TopicID())
				require.NoError(t, err)
				require.NotNil(t, topic)
				assert.Equal(t, topic.Labels, a.params.Labels)
			},
		},
		{
			"fail create with no name",
			nil,
			CreateTopicParams{},
			func(t *testing.T, err error, a *CreateTopic) {
				assert.Nil(t, a)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "name", "error message should mention that name is required")
			},
		},
		{
			"fail create duplicate",
			func(t *testing.T) {
				require.NoError(t, client.DoCtxTx(
					testutils.ContextForTest(t),
					nil,
					NewCreateTopic(CreateTopicParams{Name: "xyzzy"}).
						Execute,
				))
			},
			CreateTopicParams{
				Name: "xyzzy",
			},
			func(t *testing.T, err error, a *CreateTopic) {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrExists)
				assert.False(t, a.HasResults())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enttest.ResetTables(t, client)
			if tt.before != nil {
				tt.before(t)
			}
			var a *CreateTopic
			var err error
			topicMod := TopicModifiedAwaiter(uuid.UUID{}, tt.params.Name)
			defer CancelTopicModifiedAwaiter(uuid.UUID{}, tt.params.Name, topicMod)
			func() {
				defer func() {
					recovered := recover()
					if recovered != nil {
						err = recovered.(error)
					}
				}()
				a = NewCreateTopic(tt.params)
			}()
			if err == nil {
				err = client.DoCtxTx(ctx, nil, a.Execute)
			}
			tt.checker(t, err, a)
			if err == nil {
				assertClosed(t, topicMod)
			} else {
				assertOpenEmpty(t, topicMod)
			}
		})
	}
}
