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
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

type KillController struct {
	shutdowner fx.Shutdowner
}

func (k *KillController) Register(router gin.IRouter) error {
	rg := router.Group("/server")
	rg.POST("/kill", k.HandleKill)
	rg.POST("/shutdown", k.HandleShutdown)
	return nil
}

func (k *KillController) HandleKill(c *gin.Context) {
	c.String(http.StatusOK, "Goodbye cruel world!\n")

	// do the shutdown in the background
	go func() {
		// TODO: log any error
		_ = k.shutdowner.Shutdown(fx.ExitCode(1))
	}()
}

func (k *KillController) HandleShutdown(c *gin.Context) {
	c.String(http.StatusOK, "Daisy, Daisy, Give me your answer, do!\n")

	// do the shutdown in the background
	go func() {
		// wait a moment so the response hopefully is sent
		// TODO: clean http server shutdown should have allowed the response send
		// regardless, why isn't that working?
		<-time.After(10 * time.Millisecond)

		// TODO: log any error
		_ = k.shutdowner.Shutdown()
	}()
}
