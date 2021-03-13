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
