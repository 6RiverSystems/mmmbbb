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
	"errors"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gin-gonic/gin"
)

type OASErrorHandler func(c *gin.Context, err error)

func WithOASValidation(
	spec *openapi3.T,
	validateResponse bool,
	errorHandler OASErrorHandler,
	options *openapi3filter.Options,
) gin.HandlerFunc {
	oasRouter, err := legacy.NewRouter(spec)
	if err != nil {
		panic(fmt.Errorf("unable to create a router for the given openapi schema: %w", err))
	}
	if errorHandler == nil {
		errorHandler = DefaultOASErrorHandler
	}
	return func(c *gin.Context) {
		// TODO: prometheus metrics for validation failures

		route, pathParams, err := oasRouter.FindRoute(c.Request)
		if err != nil {
			errorHandler(c, err)
			// either aborted or we don't want to validate this request, either way
			// let gin continue what's left of the chain (if anything)
			return
		}

		// Validate request
		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    c.Request,
			PathParams: pathParams,
			Route:      route,
			Options:    options,
			// QueryParams will be auto-populated from the request url
		}
		if err := openapi3filter.ValidateRequest(c.Request.Context(), requestValidationInput); err != nil {
			errorHandler(c, err)
			return
		}

		if !validateResponse {
			// skip the rest, don't do the body capture if we don't need to
			return
		}

		// setup body capture for response validation: don't let the body pass
		// through as we may need to replace it if there are errors
		bodyCapture, realWriter := CaptureResponseBody(c, false)

		c.Next()

		if len(c.Errors) > 0 || c.IsAborted() {
			// don't do response validation on errors, but we do need to pass-through the response body
			if bodyCapture.Len() > 0 {
				_, err = realWriter.Write(bodyCapture.Bytes())
				if err != nil {
					panic(err)
				}
			}
			return
		}

		responseValidationInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: requestValidationInput,
			Status:                 c.Writer.Status(),
			Header:                 c.Writer.Header(),
			Options:                options,
			// Body is set below
		}
		responseValidationInput.SetBodyBytes(bodyCapture.Bytes())

		if err := openapi3filter.ValidateResponse(c.Request.Context(), responseValidationInput); err != nil {
			// restore the real writer for error emission
			c.Writer = realWriter
			errorHandler(c, err)
			return
		}

		// write the buffered body now that validation passed
		if body := bodyCapture.Bytes(); c.Writer.Status() != http.StatusNoContent ||
			len(body) != 0 {
			_, err = realWriter.Write(body)
			if err != nil {
				panic(err)
			}
		}
	}
}

func DefaultOASErrorHandler(c *gin.Context, err error) {
	// nolint:errcheck // return value here is just a wrapped copy of the input
	c.Error(err)
	var (
		routeErr    *routers.RouteError
		requestErr  *openapi3filter.RequestError
		responseErr *openapi3filter.ResponseError
		parseErr    *openapi3filter.ParseError
	)
	// TODO: adapt ValidationErrorEncoder to work here
	switch {
	case errors.As(err, &routeErr):
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": routeErr.Reason})
	case errors.As(err, &requestErr):
		c.AbortWithStatusJSON(
			http.StatusBadRequest,
			gin.H{"error": requestErr.Reason, "details": requestErr.Err},
		)
	case errors.As(err, &responseErr):
		c.AbortWithStatusJSON(
			http.StatusInternalServerError,
			gin.H{"error": responseErr.Reason, "details": responseErr.Err},
		)
	case errors.As(err, &parseErr):
		// TODO: this may not be right
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error(), "details": err})
	default:
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func AllowUndefinedRoutes(handler OASErrorHandler) OASErrorHandler {
	return func(c *gin.Context, err error) {
		var re *routers.RouteError
		if errors.As(err, &re) {
			// TODO: prometheus metric for this
			return
		}
		handler(c, err)
	}
}
