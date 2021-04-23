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

	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/faults"
	"go.6river.tech/mmmbbb/oas"
)

type FaultInjectorController struct {
	logger *logging.Logger
	faults *faults.Set
}

func (f *FaultInjectorController) Register(reg *registry.Registry, router gin.IRouter) error {
	// TODO: allow using a non-default fault set via dependency injection
	f.faults = faults.DefaultSet()
	f.logger = logging.GetLogger("controllers/fault-injector")

	reg.RegisterMap(router, "/faults", registry.HandlerMap{
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
		Operation: fd.Operation,
		Count:     count,
		OnFault:   errorFactory(fd.Error),
	}
	if fd.Parameters != nil {
		desc.Parameters = fd.Parameters.AdditionalProperties
	}
	f.faults.Add(desc)
	c.JSON(http.StatusCreated, toOAS(desc))
}

func toOAS(d faults.Description) oas.ConfiguredFault {
	ret := oas.ConfiguredFault{
		Operation: d.Operation,
		Count:     d.Count,
	}
	if len(d.Parameters) != 0 {
		ret.Parameters = &oas.ConfiguredFault_Parameters{
			AdditionalProperties: d.Parameters,
		}
	}
	return ret
}

var errorMap = map[oas.ErrorType]error{
	// nil entries mean things that get special handling.

	oas.ErrorType_context_Canceled:         context.Canceled,
	oas.ErrorType_context_DeadlineExceeded: context.DeadlineExceeded,

	oas.ErrorType_ent_NotFound: nil,

	// grpc errors have extra special handling and don't need to be listed here
}

func errorFactory(errorType oas.ErrorType) func(faults.Description, faults.Parameters) error {
	if strings.HasPrefix(string(errorType), "grpc.") {
		// these are generated programatically
		var c codes.Code
		// special case
		if errorType == oas.ErrorType_grpc_Canceled {
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
		panic(fmt.Errorf("Unsupported error type '%s'", errorType))
	}

	// this relies on places checking for injections using errors.Is / errors.As
	return func(d faults.Description, p faults.Parameters) error {
		err := err
		switch errorType {
		case oas.ErrorType_ent_NotFound:
			label := p["label"]
			if label == "" {
				label = "INJECTED_FAKE_ENTITY"
			}
			err = ent.NewNotFoundError(label)
		}
		if len(d.Parameters) == 0 {
			return fmt.Errorf("Injected fault matched %s(): %w", d.Operation, err)
		}
		return fmt.Errorf("Injected fault matched %s(%v): %w", d.Operation, d.Parameters, err)
	}
}
