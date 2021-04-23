package controllers

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.6river.tech/mmmbbb/faults"
	"go.6river.tech/mmmbbb/oas"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Test_errorFactory_grpc(t *testing.T) {
	// codes.Code.UnmarshalJSON will accept numeric codes, we can use this to
	// deduce the max code. we skip 0 because it's OK and not an error.
	for n := 1; ; n++ {
		s := strconv.Itoa(n)
		var c codes.Code
		if err := c.UnmarshalJSON([]byte(s)); err != nil {
			break
		}
		t.Run(c.String(), func(t *testing.T) {
			errorType := oas.ErrorType("grpc." + c.String())
			f := errorFactory(errorType)
			err := f(faults.Description{Operation: "op_" + c.String()}, nil)
			// this relies on the grpc Error implementing Is as value equality instead
			// of pointer equality
			assert.ErrorIs(t, err, status.Errorf(c, "Injected fault for op_%s()", c))
			// this will only work if the error is directly a status error, or if grpc
			// updates status.Code to use errors.As or other unwrapping capabilities.
			// upstream: https://github.com/grpc/grpc-go/issues/2934
			assert.Equal(t, c, status.Code(err))
		})
	}
}
