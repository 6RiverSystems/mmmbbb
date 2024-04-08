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

package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyPgAppName(t *testing.T) {
	tests := []struct {
		name           string
		dbUrl          string
		pod, container string
		want           string
	}{
		{
			"nothing",
			"postgres://user@host:5432/database",
			"", "",
			"postgres://user@host:5432/database",
		},
		{
			"everything",
			"postgres://user@host:5432/database",
			"__pod__", "__container__",
			"postgres://user@host:5432/database?application_name=__pod__%2F__container__",
		},
		{
			"pre-set",
			"postgres://user@host:5432/database?application_name=foo",
			"__pod__", "__container__",
			"postgres://user@host:5432/database?application_name=foo",
		},
		{
			"just pod",
			"postgres://user@host:5432/database",
			"__pod__", "",
			"postgres://user@host:5432/database?application_name=__pod__",
		},
		{
			"just container",
			"postgres://user@host:5432/database",
			"", "__container__",
			"postgres://user@host:5432/database?application_name=__container__",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldPod, oldCon := os.Getenv("POD_NAME"), os.Getenv("CONTAINER_NAME")
			os.Setenv("POD_NAME", tt.pod)
			os.Setenv("CONTAINER_NAME", tt.container)
			t.Cleanup(func() {
				os.Setenv("POD_NAME", oldPod)
				os.Setenv("CONTAINER_NAME", oldCon)
			})
			assert.Equal(t, tt.want, ApplyPgAppName(tt.dbUrl))
		})
	}
}
