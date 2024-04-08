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

package grpc

import (
	"context"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.6river.tech/mmmbbb/faults"
)

func UnaryFaultInjector(s *faults.Set) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		service, method := splitMethodName(info.FullMethod)
		params := paramsFromProtoMessage(service, method, req)
		defer paramsPool.Put(params)
		if err := s.Check(method, params); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
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

func StreamFaultInjector(s *faults.Set) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// check for a fault on the start
		service, method := splitMethodName(info.FullMethod)
		if err := s.Check(method, faults.Parameters{service: method}); err != nil {
			return err
		}
		fs := &faultingStream{ss, s, service, method}
		return handler(srv, fs)
	}
}

type faultingStream struct {
	grpc.ServerStream
	faults          *faults.Set
	service, method string
}

func (fs *faultingStream) RecvMsg(m interface{}) error {
	// check for a fault for receiving in general
	if err := fs.faults.Check(fs.method+":RecvMsg", nil); err != nil {
		return err
	}
	err := fs.ServerStream.RecvMsg(m)
	if err != nil {
		return err
	}
	// check for a fault on the received message
	params := paramsFromProtoMessage(fs.service, fs.method, m)
	defer paramsPool.Put(params)
	if err := fs.faults.Check(fs.method+":RecvMsg", params); err != nil {
		return err
	}
	return nil
}

func (fs *faultingStream) SendMsg(m interface{}) error {
	// check for a fault for sending the message
	params := paramsFromProtoMessage(fs.service, fs.method, m)
	defer paramsPool.Put(params)
	if err := fs.faults.Check(fs.method+":SendMsg", params); err != nil {
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
