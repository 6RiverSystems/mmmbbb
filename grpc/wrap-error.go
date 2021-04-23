package grpc

import (
	"fmt"

	"google.golang.org/grpc/status"
)

// workaround for grpc not supporting error unwrapping. if err seems to be a
// grpc status error, returns a new error with the same code & details, but a
// wrapped description. Otherwise wraps err normally. upstream:
// https://github.com/grpc/grpc-go/issues/2934
func WrapError(err error, format string, args ...interface{}) error {
	if s, ok := status.FromError(err); ok {
		// converting to protobuf will also clone it
		sp := s.Proto()
		args = append(args, sp.Message)
		sp.Message = fmt.Sprintf(format+": %s", args...)
		return status.ErrorProto(sp)
	}
	args = append(args, err)
	return fmt.Errorf(format+": %w", args...)
}
