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

package middleware

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

func CaptureResponseBody(c *gin.Context, passThrough bool) (*bytes.Buffer, gin.ResponseWriter) {
	bc := &bodyCapture{
		ResponseWriter: c.Writer,
		passThrough:    passThrough,
		body:           &bytes.Buffer{},
	}
	c.Writer = bc
	return bc.body, bc.ResponseWriter
}

type bodyCapture struct {
	gin.ResponseWriter
	passThrough bool
	// could use io.MultiWriter here but not worth it
	body *bytes.Buffer
}

func (bc *bodyCapture) Write(b []byte) (int, error) {
	// Buffer.Write never returns an error
	bc.body.Write(b)
	if bc.passThrough {
		return bc.ResponseWriter.Write(b)
	} else {
		return len(b), nil
	}
}

func (bc *bodyCapture) WriteString(s string) (int, error) {
	// Buffer.Write never returns an error
	bc.body.WriteString(s)
	if bc.passThrough {
		return bc.ResponseWriter.WriteString(s)
	} else {
		return len(s), nil
	}
}
