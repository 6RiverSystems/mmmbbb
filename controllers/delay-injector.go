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
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"go.6river.tech/gosix/db"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/subscription"
	"go.6river.tech/mmmbbb/middleware"
	"go.6river.tech/mmmbbb/oas"
)

type DelayInjectorController struct{}

func (cc *DelayInjectorController) Register(reg *registry.Registry, router gin.IRouter) error {
	rg := router.Group("/delays")
	rg.Use(middleware.WithTransaction(db.GetDefaultDbName(), &sql.TxOptions{}))

	rg.GET("/*subscription", cc.GetDelay)
	rg.PUT("/*subscription", cc.PutDelay)
	rg.DELETE("/*subscription", cc.DeleteDelay)

	return nil
}

func (cc *DelayInjectorController) GetDelay(c *gin.Context) {
	subName := c.Param("subscription")
	// Gin includes the leading slash when we're using the *param format
	subName = strings.TrimPrefix(subName, "/")
	tx := middleware.Transaction(c, db.GetDefaultDbName())

	sub, err := tx.Subscription.Query().Where(
		subscription.Name(subName),
		subscription.DeletedAtIsNil(),
	).Only(c)
	if err != nil {
		if ent.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"subscription": subName, "message": "Subscription not found"})
			return
		}
		panic(err)
	}

	c.JSON(http.StatusOK, oas.DeliveryDelay{Delay: sub.DeliveryDelay})
}

func (cc *DelayInjectorController) PutDelay(c *gin.Context) {
	subName := c.Param("subscription")
	// Gin includes the leading slash when we're using the *param format
	subName = strings.TrimPrefix(subName, "/")
	tx := middleware.Transaction(c, db.GetDefaultDbName())

	defer c.Request.Body.Close()
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var dd oas.DeliveryDelay
	if err := decoder.Decode(&dd); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) // nolint:errcheck
		return
	}
	if dd.Delay < 0 {
		c.JSON(http.StatusBadRequest, fmt.Errorf("delay must be >= 0"))
		return
	}

	n, err := tx.Subscription.Update().
		SetDeliveryDelay(dd.Delay).
		Where(
			subscription.Name(subName),
			subscription.DeletedAtIsNil(),
		).Save(c)
	if err != nil {
		panic(err)
	}
	if n <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"subscription": subName, "message": "Subscription not found"})
	}
	if n > 1 {
		panic(fmt.Errorf("BUG DETECTED: %d live subs with same name '%s'", n, subName))
	}

	c.JSON(http.StatusOK, oas.DeliveryDelay{Delay: dd.Delay})
}

func (cc *DelayInjectorController) DeleteDelay(c *gin.Context) {
	subName := c.Param("subscription")
	// Gin includes the leading slash when we're using the *param format
	subName = strings.TrimPrefix(subName, "/")
	tx := middleware.Transaction(c, db.GetDefaultDbName())

	n, err := tx.Subscription.Update().
		SetDeliveryDelay(0).
		Where(
			subscription.Name(subName),
			subscription.DeletedAtIsNil(),
		).Save(c)
	if err != nil {
		panic(err)
	}
	if n <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"subscription": subName, "message": "Subscription not found"})
	}
	if n > 0 {
		panic(fmt.Errorf("BUG DETECTED: %d live subs with same name '%s'", n, subName))
	}

	c.Status(http.StatusNoContent)
}
