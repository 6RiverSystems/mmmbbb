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

package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
)

// nolint:unused,deadcode // want to save this for future work
func handleActionError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, actions.ErrNotFound):
		c.AbortWithError(http.StatusNotFound, err) // nolint:errcheck
	case ent.IsNotFound(err):
		c.AbortWithError(http.StatusNotFound, err) // nolint:errcheck
	case errors.Is(err, actions.ErrExists):
		c.AbortWithError(http.StatusConflict, err) // nolint:errcheck
	case errors.Is(err, context.Canceled):
		// generally means the client went away, so what we do here is almost irrelevant
		c.AbortWithError(http.StatusInternalServerError, err) // nolint:errcheck
	case errors.Is(err, context.DeadlineExceeded):
		c.AbortWithError(http.StatusRequestTimeout, err) // nolint:errcheck
	default:
		return false
	}
	return true
}
