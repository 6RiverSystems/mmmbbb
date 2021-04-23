package grpc

import (
	"context"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.6river.tech/mmmbbb/faults"
)

func UnaryFaultInjection(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	service, method := splitMethodName(info.FullMethod)
	params := paramsFromProtoMessage(service, method, req)
	defer paramsPool.Put(params)
	if err := faults.Check(method, params); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// allocating and freeing lots of maps is expensive, using a pool cuts the
// runtime of paramsFromProtoMessage by almost two thirds
var paramsPool = &sync.Pool{
	New: func() interface{} { return make(faults.Parameters) },
}

func paramsFromProtoMessage(service, method string, req interface{}) faults.Parameters {
	params := paramsPool.Get().(faults.Parameters)
	for k := range params {
		delete(params, k)
	}
	// put service:method as a parameter so any ambiguities can be resolved if
	// needed
	params[service] = method
	if pr, ok := req.(protoreflect.ProtoMessage); ok {
		m := pr.ProtoReflect()
		params[service] = method
		m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			if fd.Kind() == protoreflect.StringKind && fd.Cardinality() != protoreflect.Repeated {
				sv := v.String()
				// allow matching by any version of the field name
				tn := fd.TextName()
				params[tn] = sv
				// JSONName and Name are usually the same as TextName, equality checks
				// are faster than map collisions
				if jn := fd.JSONName(); jn != tn {
					params[fd.JSONName()] = sv
				}
				if nn := string(fd.Name()); nn != tn {
					params[nn] = sv
				}
				params[string(fd.FullName())] = sv
			}
			return true
		})
	}
	return params
}

func StreamFaultInjection(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	// check for a fault on the start
	service, method := splitMethodName(info.FullMethod)
	if err := faults.Check(method, faults.Parameters{service: method}); err != nil {
		return err
	}
	fs := &faultingStream{ss, service, method}
	return handler(srv, fs)
}

type faultingStream struct {
	grpc.ServerStream
	service, method string
}

func (fs *faultingStream) RecvMsg(m interface{}) error {
	// check for a fault for receiving in general
	if err := faults.Check(fs.method+":RecvMsg", nil); err != nil {
		return err
	}
	err := fs.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}
	// check for a fault on the received message
	params := paramsFromProtoMessage(fs.service, fs.method, m)
	defer paramsPool.Put(params)
	if err := faults.Check(fs.method+":RecvMsg", params); err != nil {
		return err
	}
	return nil
}

func (fs *faultingStream) SendMsg(m interface{}) error {
	// check for a fault for sending the message
	params := paramsFromProtoMessage(fs.service, fs.method, m)
	defer paramsPool.Put(params)
	if err := faults.Check(fs.method+":SendMsg", params); err != nil {
		return err
	}
	err := fs.ServerStream.SendMsg(m)
	if err != nil {
		return err
	}
	return nil
}

// copied from grpc_prometheus (Apache licensed)
func splitMethodName(fullMethodName string) (service, method string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "unknown", "unknown"
}
