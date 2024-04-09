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
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"go.6river.tech/mmmbbb/logging"
)

type LogController struct {
	// no own logger for this one
}

const apiRoot = "/logcontrol"

func (lc *LogController) Register(router gin.IRouter) error {
	Register(router, apiRoot, HandlerMap{
		{http.MethodGet, ""}:                  lc.GetLevels,
		{http.MethodGet, "/"}:                 lc.GetLevels,
		{http.MethodGet, "/:component"}:       lc.GetLevels,
		{http.MethodPut, "/:component/level"}: lc.SetComponentLevel,
	})
	return nil
}

func (*LogController) GetLevels(c *gin.Context) {
	// not worth streaming this one, though we could.

	// this handles both all and tree-limited requests
	root, _ := c.Params.Get("component")

	// despite zerolog.Level implementing Stringer, it gets JSON serialized as its
	// underlying numeric value
	var results []struct{ Component, Level string }
	for component, level := range logging.ComponentLevels() {
		if len(root) != 0 && component != root && !strings.HasPrefix(component, root+"/") {
			continue
		}
		results = append(results, struct{ Component, Level string }{
			Component: component,
			Level:     level.String(),
		})
	}
	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "No matching loggers"})
	} else {
		c.JSON(http.StatusOK, results)
	}
}

func (lc *LogController) SetComponentLevel(c *gin.Context) {
	root := c.Param("component")
	children := false
	childrenStr := c.Param("children")
	if childrenStr != "" {
		var err error
		children, err = strconv.ParseBool(childrenStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"children": childrenStr, "message": err.Error()})
			return
		}
	}

	levelStr, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// will be recovered
		panic(err)
	}
	level, err := zerolog.ParseLevel(string(levelStr))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"level": levelStr, "message": err.Error()})
		return
	}

	logging.SetComponentLevel(root, children, level)

	lc.GetLevels(c)
}
