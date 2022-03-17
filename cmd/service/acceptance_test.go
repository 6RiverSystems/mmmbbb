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

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.6river.tech/gosix/db/postgres"
	"go.6river.tech/gosix/logging"
	oastypes "go.6river.tech/gosix/oas"
	"go.6river.tech/gosix/server"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/oas"
)

func checkMainError(t *testing.T, err error) {
	t.Helper()
	// main died, errgroup will have called skip or fatal
	// if db doesn't exist, count this as a skip instead of a fail
	if _, ok := postgres.IsPostgreSQLErrorCode(err, postgres.InvalidCatalogName); ok {
		// we can't call skip from here, we need to check this again higher up
		t.Skip("Acceptance test DB does not exist, skipping test")
	}
	require.NoError(t, err, "main() should not panic")
}

func mustJSON(t *testing.T, value interface{}) []byte {
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}

func mustBase64(t *testing.T, value []byte) string {
	return base64.StdEncoding.EncodeToString(value)
}

func mustUnBase64(t *testing.T, encoded string) []byte {
	value, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	return value
}

func TestEndpoints(t *testing.T) {
	logging.ConfigureDefaultLogging()
	server.EnableRandomPorts()

	// setup server
	oldEnv := os.Getenv("NODE_ENV")
	defer os.Setenv("NODE_ENV", oldEnv)
	// this will target a postgresql db by default
	os.Setenv("NODE_ENV", "acceptance")
	eg, ctx := errgroup.WithContext(testutils.ContextForTest(t))
	app := NewApp()
	eg.Go(app.Main)

	client := http.DefaultClient
	baseUrl := "http://localhost:" + strconv.Itoa(server.ResolvePort(defaults.Port, 0))

	// wait for app to start
	for {
		delay := time.After(time.Millisecond)
		select {
		case <-ctx.Done():
			checkMainError(t, eg.Wait())
			return
		case <-delay:
			// continue with checks...
		}

		if !app.Registry.ServicesStarted() {
			continue
		}
		if err := app.Registry.WaitAllReady(ctx); err != nil {
			checkMainError(t, eg.Wait())
		}
		break
	}

	// reset old db records
	ec, ok := app.EntClient()
	require.True(t, ok)
	enttest.ResetTables(t, ec.(*ent.Client))

	// load the OAS spec
	swagger := oas.MustLoadSpec()

	// TODO: verify elements of the swagger spec
	assert.NotNil(t, swagger)

	// use uuid to generate a unique string so our create calls cannot collide
	uniqueProject := uuid.New().String()
	uniqueTopic := uuid.New().String()
	uniqueSubscription := uuid.New().String()
	// need to store this across tests
	var ackId uuid.UUID

	type msi = map[string]interface{}

	tests := []struct {
		name        string
		url         string
		method      string
		bodyBuilder func(*testing.T) json.RawMessage
		check       func(*testing.T, *http.Response)
	}{
		{
			"uptime",
			"/",
			http.MethodGet,
			nil,
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyData, err := io.ReadAll(resp.Body)
				assert.NoError(t, err)
				bodyObject := msi{}
				err = json.Unmarshal(bodyData, &bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "startTime")
				require.IsType(t, "", bodyObject["startTime"])
				startTime, err := time.Parse(oastypes.OASRFC3339Millis, bodyObject["startTime"].(string))
				assert.NoError(t, err)
				assert.NotZero(t, startTime)
			},
		},

		{
			"create topic",
			"/v1/projects/" + uniqueProject + "/topics/" + uniqueTopic,
			http.MethodPut,
			nil,
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "name")
				assert.IsType(t, "", bodyObject["name"])
				assert.Equal(t, "projects/"+uniqueProject+"/topics/"+uniqueTopic, bodyObject["name"])
				assert.Contains(t, bodyObject, "labels")
				assert.IsType(t, msi{}, bodyObject["labels"])
				assert.Empty(t, bodyObject["labels"])
			},
		},
		{
			"create subscription",
			fmt.Sprintf("/v1/projects/%s/subscriptions/%s", uniqueProject, uniqueSubscription),
			http.MethodPut,
			func(t *testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"topic":                    fmt.Sprintf("projects/%s/topics/%s", uniqueProject, uniqueTopic),
					"messageRetentionDuration": "3600s",
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "name")
				assert.IsType(t, "", bodyObject["name"])
				assert.Equal(t, bodyObject["name"], "projects/"+uniqueProject+"/subscriptions/"+uniqueSubscription)
				assert.Contains(t, bodyObject, "topic")
				assert.IsType(t, "", bodyObject["topic"])
				assert.Equal(t, bodyObject["topic"], "projects/"+uniqueProject+"/topics/"+uniqueTopic)
				assert.Contains(t, bodyObject, "labels")
				assert.IsType(t, msi{}, bodyObject["labels"])
				assert.Empty(t, bodyObject["labels"])
				assert.Contains(t, bodyObject, "expirationPolicy")
				assert.IsType(t, msi{}, bodyObject["expirationPolicy"])
				assert.Contains(t, bodyObject["expirationPolicy"], "ttl")
				assert.IsType(t, "", bodyObject["expirationPolicy"].(msi)["ttl"])
				assert.Equal(t, fmt.Sprintf("%ds", 60*60*24*30), bodyObject["expirationPolicy"].(msi)["ttl"])
			},
		},
		{
			"publish message",
			fmt.Sprintf("/v1/projects/%s/topics/%s:publish", uniqueProject, uniqueTopic),
			http.MethodPost,
			func(t *testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"messages": []msi{
						{
							"data": mustBase64(t, mustJSON(t, msi{
								"hello": "world",
							})),
						},
					},
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "messageIds")
				require.IsType(t, []interface{}{}, bodyObject["messageIds"])
				ids := bodyObject["messageIds"].([]interface{})
				assert.Len(t, ids, 1)
				assert.IsType(t, "", ids[0])
				id, err := uuid.Parse(ids[0].(string))
				assert.NoError(t, err)
				assert.NotZero(t, id)
			},
		},
		{
			"receive message",
			fmt.Sprintf("/v1/projects/%s/subscriptions/%s:pull", uniqueProject, uniqueSubscription),
			http.MethodPost,
			func(t *testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"returnImmediately": true,
					"maxMessages":       1,
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "receivedMessages")
				require.IsType(t, []interface{}{}, bodyObject["receivedMessages"])
				msgs := bodyObject["receivedMessages"].([]interface{})
				if assert.Len(t, msgs, 1) {
					assert.IsType(t, msi{}, msgs[0])
					rMsg := msgs[0].(msi)
					assert.Contains(t, rMsg, "ackId")
					assert.IsType(t, "", rMsg["ackId"])
					ackId, err = uuid.Parse(rMsg["ackId"].(string))
					assert.NoError(t, err)
					assert.NotZero(t, ackId)
					assert.Contains(t, rMsg, "message")
					assert.IsType(t, msi{}, rMsg["message"])
					msg := rMsg["message"].(msi)
					assert.Contains(t, msg, "publishTime")
					assert.IsType(t, "", msg["publishTime"])
					_, err = time.Parse(time.RFC3339Nano, msg["publishTime"].(string))
					assert.NoError(t, err)
					assert.IsType(t, "", msg["data"])
					data := mustUnBase64(t, msg["data"].(string))
					payload := msi{}
					err = json.Unmarshal(data, &payload)
					assert.NoError(t, err)
					assert.Equal(t, payload, msi{"hello": "world"})
				}
			},
		},
		{
			"ack message",
			fmt.Sprintf("/v1/projects/%s/subscriptions/%s:acknowledge", uniqueProject, uniqueSubscription),
			http.MethodPost,
			func(*testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"ackIds": []uuid.UUID{ackId},
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				// gRPC API doesn't say how many ACKs were "successful"
				assert.Empty(t, bodyObject)
			},
		},
		{
			"receive no message",
			fmt.Sprintf("/v1/projects/%s/subscriptions/%s:pull", uniqueProject, uniqueSubscription),
			http.MethodPost,
			func(t *testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"returnImmediately": true,
					"maxMessages":       1,
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				assert.Contains(t, bodyObject, "receivedMessages")
				require.IsType(t, []interface{}{}, bodyObject["receivedMessages"])
				msgs := bodyObject["receivedMessages"].([]interface{})
				assert.Empty(t, msgs)
			},
		},
		{
			"re-ack message",
			fmt.Sprintf("/v1/projects/%s/subscriptions/%s:acknowledge", uniqueProject, uniqueSubscription),
			http.MethodPost,
			func(*testing.T) json.RawMessage {
				return mustJSON(t, msi{
					"ackIds": []uuid.UUID{ackId},
				})
			},
			func(t *testing.T, resp *http.Response) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Contains(t, resp.Header.Get("Content-type"), "application/json")
				bodyObject := msi{}
				err := json.NewDecoder(resp.Body).Decode(&bodyObject)
				assert.NoError(t, err)
				// gRPC API doesn't say how many ACKs were "successful"
				assert.Empty(t, bodyObject)
			},
		},

		{
			"shutdown",
			"/server/shutdown",
			http.MethodPost,
			nil,
			nil,
		},
	}

	// run tests, last one will close app
	for i, tt := range tests {
		if tt.name == "" {
			tt.name = tt.method + " " + tt.url
		}
		// if a test fails, skip everything until the final shutdown "test"
		skipInner := t.Failed() && i < len(tests)-1
		t.Run(tt.name, func(t *testing.T) {
			if skipInner {
				t.Skip("intermediate test failed")
			}
			var bodyReader io.Reader
			if tt.bodyBuilder != nil {
				body := tt.bodyBuilder(t)
				if body != nil {
					bodyReader = bytes.NewReader(body)
				}
			}
			// ctx is from the errgroup for running main(), so if that blows up the
			// request (and thus tests) get canceled instead of hanging until the
			// timeout.
			req, err := http.NewRequestWithContext(ctx, tt.method, baseUrl+tt.url, bodyReader)
			require.NoError(t, err)
			if bodyReader != nil {
				req.Header.Add("Content-Type", "application/json")
			}
			resp, err := client.Do(req)
			if resp != nil {
				defer resp.Body.Close()
			}
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, resp)
			}
		})
	}

	app.Registry.RequestStopServices()
	err := eg.Wait()
	assert.NoError(t, err, "main should not panic")
}
