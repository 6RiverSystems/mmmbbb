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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/iancoleman/strcase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/faults"
	"go.6river.tech/mmmbbb/logging"
	"go.6river.tech/mmmbbb/oas"
)

// TODO: would like to move this to gosix, but that requires generalizing both
// the ent and oas interactions

type FaultInjectorController struct {
	logger *logging.Logger
	faults *faults.Set
}

func (f *FaultInjectorController) Register(router gin.IRouter) error {
	f.logger = logging.GetLogger("controllers/fault-injector")

	Register(router, "/faults", HandlerMap{
		{http.MethodGet, ""}:         f.GetDescriptors,
		{http.MethodGet, "/"}:        f.GetDescriptors,
		{http.MethodPost, "/inject"}: f.AddDescriptor,
	})
	return nil
}

func (f *FaultInjectorController) GetDescriptors(c *gin.Context) {
	current := f.faults.Current()
	// must not be nil, even if empty
	result := []oas.ConfiguredFault{}
	for _, l := range current {
		for _, d := range l {
			result = append(result, toOAS(d))
		}
	}
	c.JSON(http.StatusOK, result)
}

func (f *FaultInjectorController) AddDescriptor(c *gin.Context) {
	defer c.Request.Body.Close()
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var fd oas.FaultDescription
	if err := decoder.Decode(&fd); err != nil {
		c.AbortWithError(http.StatusBadRequest, err) // nolint:errcheck
		return
	}

	count := int64(9223372036854775807) // max int64
	if fd.Count != nil {
		count = *fd.Count
	}

	desc := faults.Description{
		Operation:        fd.Operation,
		Count:            count,
		OnFault:          errorFactory(fd.Error),
		FaultDescription: string(fd.Error),
	}
	if fd.Parameters != nil {
		desc.Parameters = *fd.Parameters
	}
	f.faults.Add(desc)
	c.JSON(http.StatusCreated, toOAS(desc))
}

func toOAS(d faults.Description) oas.ConfiguredFault {
	ret := oas.ConfiguredFault{
		Operation:        d.Operation,
		Count:            d.Count,
		FaultDescription: &d.FaultDescription,
	}
	if len(d.Parameters) != 0 {
		ret.Parameters = &d.Parameters
	}
	return ret
}

var errorMap = map[oas.ErrorType]error{
	// nil entries mean things that get special handling.

	oas.ContextCanceled:         context.Canceled,
	oas.ContextDeadlineExceeded: context.DeadlineExceeded,

	oas.EntNotFound: nil,

	// grpc errors have extra special handling and don't need to be listed here
}

func errorFactory(errorType oas.ErrorType) func(faults.Description, faults.Parameters) error {
	if strings.HasPrefix(string(errorType), "grpc.") {
		// these are generated programatically
		var c codes.Code
		// special case
		if errorType == oas.GrpcCanceled {
			c = codes.Canceled
		} else {
			s := strcase.ToScreamingSnake(strings.TrimPrefix(string(errorType), "grpc."))
			if b, e := json.Marshal(s); e != nil {
				panic(e)
			} else if e := c.UnmarshalJSON(b); e != nil {
				panic(e)
			}
		}
		return func(d faults.Description, p faults.Parameters) error {
			if len(d.Parameters) == 0 {
				return status.Errorf(c, "Injected fault matched %s()", d.Operation)
			}
			return status.Errorf(c, "Injected fault matched %s(%v)", d.Operation, d.Parameters)
		}
	}

	var err error
	err, ok := errorMap[errorType]
	if !ok {
		panic(fmt.Errorf("unsupported error type '%s'", errorType))
	}

	// this relies on places checking for injections using errors.Is / errors.As
	return func(d faults.Description, p faults.Parameters) error {
		err := err
		switch errorType {
		case oas.EntNotFound:
			label := p["label"]
			if label == "" {
				label = "INJECTED_FAKE_ENTITY"
			}
			err = ent.NewNotFoundError(label)
		}
		if len(d.Parameters) == 0 {
			return fmt.Errorf("injected fault matched %s(): %w", d.Operation, err)
		}
		return fmt.Errorf("injected fault matched %s(%v): %w", d.Operation, d.Parameters, err)
	}
}
