// Copyright (c) 2022 6 River Systems
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

package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.6river.tech/gosix/ent/customtypes"
	"go.6river.tech/gosix/ginmiddleware"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/oas"
)

func setupDelayInjectorController(
	t *testing.T,
	dbName string,
	ec *ent.Client,
	cc *DelayInjectorController,
) *httptest.Server {
	enttest.ResetTables(t, ec)
	r := gin.New()
	r.Use(ginmiddleware.WithEntClient(ec, dbName))
	assert.NoError(t, cc.Register(nil, r))
	s := httptest.NewServer(r)
	return s
}

func createDelaySub(dd time.Duration) func(*testing.T, *ent.Client) {
	return func(t *testing.T, c *ent.Client) {
		ctx := testutils.ContextForTest(t)
		topic, err := c.Topic.Create().
			SetName(t.Name()).
			Save(ctx)
		require.NoError(t, err)
		_, err = c.Subscription.Create().
			SetName(t.Name()).
			SetTopic(topic).
			SetDeliveryDelay(customtypes.Interval(dd)).
			SetExpiresAt(time.Now().Add(time.Hour)).
			SetTTL(customtypes.Interval(time.Hour)).
			SetMessageTTL(customtypes.Interval(time.Hour)).
			Save(ctx)
		require.NoError(t, err)
	}
}

func assertDelaySubFound(dd time.Duration) func(*testing.T, *http.Response, error) {
	return func(t *testing.T, r *http.Response, err error) {
		assert.NoError(t, err)
		if assert.NotNil(t, r) {
			assert.Equal(t, http.StatusOK, r.StatusCode)
			ct := r.Header.Get("content-type")
			mt, _, err := mime.ParseMediaType(ct)
			assert.NoError(t, err)
			assert.Equal(t, "application/json", mt)
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var odd oas.DeliveryDelay
			assert.NoError(t, json.Unmarshal(body, &odd))
			assert.Equal(t, customtypes.Interval(dd), odd.Delay)
		}
	}
}

func assertDelaySubFoundDB(dd time.Duration) func(*testing.T, *ent.Client) {
	return func(t *testing.T, c *ent.Client) {
		ctx := testutils.ContextForTest(t)
		sub, err := c.Subscription.Query().
			Where(subscription.Name(t.Name())).
			Only(ctx)
		if assert.NoError(t, err) {
			assert.Equal(t, sub.DeliveryDelay, customtypes.Interval(dd))
		}
	}
}

func assertDelaySubNotFound(t *testing.T, r *http.Response, err error) {
	assert.NoError(t, err)
	if assert.NotNil(t, r) {
		assert.Equal(t, http.StatusNotFound, r.StatusCode)
		ct := r.Header.Get("content-type")
		mt, _, err := mime.ParseMediaType(ct)
		assert.NoError(t, err)
		assert.Equal(t, "application/json", mt)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		var b gin.H
		assert.NoError(t, json.Unmarshal(body, &b))
		assert.Contains(t, b, "subscription")
		assert.Contains(t, b, "message")
		assert.Equal(t, b["subscription"], t.Name())
	}
}

func assertNoContent(t *testing.T, r *http.Response, err error) {
	assert.NoError(t, err)
	if assert.NotNil(t, r) {
		assert.Equal(t, http.StatusNoContent, r.StatusCode)
		assert.Len(t, r.Header.Values("content-type"), 0)
		assert.Equal(t, http.NoBody, r.Body)
	}
}

func TestDelayInjectorController_GetDelay(t *testing.T) {
	type test struct {
		name  string
		setup func(*testing.T, *ent.Client)
		check func(*testing.T, *http.Response, error)
	}

	tests := []test{
		{
			"not found",
			nil,
			assertDelaySubNotFound,
		},
		{
			"zero delay",
			createDelaySub(0),
			assertDelaySubFound(0),
		},
		{
			"1s delay",
			createDelaySub(time.Second),
			assertDelaySubFound(time.Second),
		},
		{
			"90s delay",
			createDelaySub(90 * time.Second),
			assertDelaySubFound(90 * time.Second),
		},
		{
			"36h delay",
			createDelaySub(36 * time.Hour),
			assertDelaySubFound(36 * time.Hour),
		},
	}

	client := enttest.ClientForTest(t)
	dbName := t.Name()
	cc := &DelayInjectorController{dbName: dbName}
	s := setupDelayInjectorController(t, dbName, client, cc)
	defer s.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t, client)
			}
			ctx := testutils.ContextForTest(t)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+path.Join(delaysRootPath, t.Name()), nil)
			assert.NoError(t, err)
			cli := s.Client()
			resp, err := cli.Do(req)
			tt.check(t, resp, err)
		})
	}
}

func TestDelayInjectorController_SetDelay(t *testing.T) {
	oasBody := func(dd time.Duration) func(*testing.T, *http.Request) {
		return func(t *testing.T, r *http.Request) {
			body, err := json.Marshal(oas.DeliveryDelay{Delay: customtypes.Interval(dd)})
			if assert.NoError(t, err) {
				r.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
	}

	type test struct {
		name     string
		setup    func(*testing.T, *ent.Client)
		req      func(*testing.T, *http.Request)
		checkReq func(*testing.T, *http.Response, error)
		checkDb  func(*testing.T, *ent.Client)
	}

	tests := []test{
		{
			"not found",
			nil,
			oasBody(0),
			assertDelaySubNotFound,
			nil,
		},
		{
			"set zero to 1m",
			createDelaySub(0),
			oasBody(time.Minute),
			assertDelaySubFound(time.Minute),
			assertDelaySubFoundDB(time.Minute),
		},
		{
			"set 1m to zero",
			createDelaySub(time.Minute),
			oasBody(0),
			assertDelaySubFound(0),
			assertDelaySubFoundDB(0),
		},
	}

	client := enttest.ClientForTest(t)
	dbName := t.Name()
	cc := &DelayInjectorController{dbName: dbName}
	s := setupDelayInjectorController(t, dbName, client, cc)
	defer s.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t, client)
			}
			ctx := testutils.ContextForTest(t)
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.URL+path.Join(delaysRootPath, t.Name()), nil)
			assert.NoError(t, err)
			tt.req(t, req)
			cli := s.Client()
			resp, err := cli.Do(req)
			tt.checkReq(t, resp, err)
		})
	}
}

func TestDelayInjectorController_DeleteDelay(t *testing.T) {
	type test struct {
		name     string
		setup    func(*testing.T, *ent.Client)
		checkReq func(*testing.T, *http.Response, error)
		checkDb  func(*testing.T, *ent.Client)
	}

	tests := []test{
		{
			"not found",
			nil,
			assertDelaySubNotFound,
			nil,
		},
		{
			"set 1m to zero",
			createDelaySub(time.Minute),
			assertNoContent,
			assertDelaySubFoundDB(0),
		},
	}

	client := enttest.ClientForTest(t)
	dbName := t.Name()
	cc := &DelayInjectorController{dbName: dbName}
	s := setupDelayInjectorController(t, dbName, client, cc)
	defer s.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t, client)
			}
			ctx := testutils.ContextForTest(t)
			req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.URL+path.Join(delaysRootPath, t.Name()), nil)
			assert.NoError(t, err)
			cli := s.Client()
			resp, err := cli.Do(req)
			tt.checkReq(t, resp, err)
		})
	}
}
