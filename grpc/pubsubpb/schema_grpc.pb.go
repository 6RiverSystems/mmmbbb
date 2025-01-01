// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v3.12.4
// source: google/pubsub/v1/schema.proto

package pubsubpb

import (
	context "context"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	SchemaService_CreateSchema_FullMethodName         = "/google.pubsub.v1.SchemaService/CreateSchema"
	SchemaService_GetSchema_FullMethodName            = "/google.pubsub.v1.SchemaService/GetSchema"
	SchemaService_ListSchemas_FullMethodName          = "/google.pubsub.v1.SchemaService/ListSchemas"
	SchemaService_ListSchemaRevisions_FullMethodName  = "/google.pubsub.v1.SchemaService/ListSchemaRevisions"
	SchemaService_CommitSchema_FullMethodName         = "/google.pubsub.v1.SchemaService/CommitSchema"
	SchemaService_RollbackSchema_FullMethodName       = "/google.pubsub.v1.SchemaService/RollbackSchema"
	SchemaService_DeleteSchemaRevision_FullMethodName = "/google.pubsub.v1.SchemaService/DeleteSchemaRevision"
	SchemaService_DeleteSchema_FullMethodName         = "/google.pubsub.v1.SchemaService/DeleteSchema"
	SchemaService_ValidateSchema_FullMethodName       = "/google.pubsub.v1.SchemaService/ValidateSchema"
	SchemaService_ValidateMessage_FullMethodName      = "/google.pubsub.v1.SchemaService/ValidateMessage"
)

// SchemaServiceClient is the client API for SchemaService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Service for doing schema-related operations.
type SchemaServiceClient interface {
	// Creates a schema.
	CreateSchema(ctx context.Context, in *CreateSchemaRequest, opts ...grpc.CallOption) (*Schema, error)
	// Gets a schema.
	GetSchema(ctx context.Context, in *GetSchemaRequest, opts ...grpc.CallOption) (*Schema, error)
	// Lists schemas in a project.
	ListSchemas(ctx context.Context, in *ListSchemasRequest, opts ...grpc.CallOption) (*ListSchemasResponse, error)
	// Lists all schema revisions for the named schema.
	ListSchemaRevisions(ctx context.Context, in *ListSchemaRevisionsRequest, opts ...grpc.CallOption) (*ListSchemaRevisionsResponse, error)
	// Commits a new schema revision to an existing schema.
	CommitSchema(ctx context.Context, in *CommitSchemaRequest, opts ...grpc.CallOption) (*Schema, error)
	// Creates a new schema revision that is a copy of the provided revision_id.
	RollbackSchema(ctx context.Context, in *RollbackSchemaRequest, opts ...grpc.CallOption) (*Schema, error)
	// Deletes a specific schema revision.
	DeleteSchemaRevision(ctx context.Context, in *DeleteSchemaRevisionRequest, opts ...grpc.CallOption) (*Schema, error)
	// Deletes a schema.
	DeleteSchema(ctx context.Context, in *DeleteSchemaRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// Validates a schema.
	ValidateSchema(ctx context.Context, in *ValidateSchemaRequest, opts ...grpc.CallOption) (*ValidateSchemaResponse, error)
	// Validates a message against a schema.
	ValidateMessage(ctx context.Context, in *ValidateMessageRequest, opts ...grpc.CallOption) (*ValidateMessageResponse, error)
}

type schemaServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSchemaServiceClient(cc grpc.ClientConnInterface) SchemaServiceClient {
	return &schemaServiceClient{cc}
}

func (c *schemaServiceClient) CreateSchema(ctx context.Context, in *CreateSchemaRequest, opts ...grpc.CallOption) (*Schema, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schema)
	err := c.cc.Invoke(ctx, SchemaService_CreateSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) GetSchema(ctx context.Context, in *GetSchemaRequest, opts ...grpc.CallOption) (*Schema, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schema)
	err := c.cc.Invoke(ctx, SchemaService_GetSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) ListSchemas(ctx context.Context, in *ListSchemasRequest, opts ...grpc.CallOption) (*ListSchemasResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListSchemasResponse)
	err := c.cc.Invoke(ctx, SchemaService_ListSchemas_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) ListSchemaRevisions(ctx context.Context, in *ListSchemaRevisionsRequest, opts ...grpc.CallOption) (*ListSchemaRevisionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ListSchemaRevisionsResponse)
	err := c.cc.Invoke(ctx, SchemaService_ListSchemaRevisions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) CommitSchema(ctx context.Context, in *CommitSchemaRequest, opts ...grpc.CallOption) (*Schema, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schema)
	err := c.cc.Invoke(ctx, SchemaService_CommitSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) RollbackSchema(ctx context.Context, in *RollbackSchemaRequest, opts ...grpc.CallOption) (*Schema, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schema)
	err := c.cc.Invoke(ctx, SchemaService_RollbackSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) DeleteSchemaRevision(ctx context.Context, in *DeleteSchemaRevisionRequest, opts ...grpc.CallOption) (*Schema, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Schema)
	err := c.cc.Invoke(ctx, SchemaService_DeleteSchemaRevision_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) DeleteSchema(ctx context.Context, in *DeleteSchemaRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, SchemaService_DeleteSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) ValidateSchema(ctx context.Context, in *ValidateSchemaRequest, opts ...grpc.CallOption) (*ValidateSchemaResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ValidateSchemaResponse)
	err := c.cc.Invoke(ctx, SchemaService_ValidateSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *schemaServiceClient) ValidateMessage(ctx context.Context, in *ValidateMessageRequest, opts ...grpc.CallOption) (*ValidateMessageResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ValidateMessageResponse)
	err := c.cc.Invoke(ctx, SchemaService_ValidateMessage_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SchemaServiceServer is the server API for SchemaService service.
// All implementations must embed UnimplementedSchemaServiceServer
// for forward compatibility.
//
// Service for doing schema-related operations.
type SchemaServiceServer interface {
	// Creates a schema.
	CreateSchema(context.Context, *CreateSchemaRequest) (*Schema, error)
	// Gets a schema.
	GetSchema(context.Context, *GetSchemaRequest) (*Schema, error)
	// Lists schemas in a project.
	ListSchemas(context.Context, *ListSchemasRequest) (*ListSchemasResponse, error)
	// Lists all schema revisions for the named schema.
	ListSchemaRevisions(context.Context, *ListSchemaRevisionsRequest) (*ListSchemaRevisionsResponse, error)
	// Commits a new schema revision to an existing schema.
	CommitSchema(context.Context, *CommitSchemaRequest) (*Schema, error)
	// Creates a new schema revision that is a copy of the provided revision_id.
	RollbackSchema(context.Context, *RollbackSchemaRequest) (*Schema, error)
	// Deletes a specific schema revision.
	DeleteSchemaRevision(context.Context, *DeleteSchemaRevisionRequest) (*Schema, error)
	// Deletes a schema.
	DeleteSchema(context.Context, *DeleteSchemaRequest) (*empty.Empty, error)
	// Validates a schema.
	ValidateSchema(context.Context, *ValidateSchemaRequest) (*ValidateSchemaResponse, error)
	// Validates a message against a schema.
	ValidateMessage(context.Context, *ValidateMessageRequest) (*ValidateMessageResponse, error)
	mustEmbedUnimplementedSchemaServiceServer()
}

// UnimplementedSchemaServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSchemaServiceServer struct{}

func (UnimplementedSchemaServiceServer) CreateSchema(context.Context, *CreateSchemaRequest) (*Schema, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateSchema not implemented")
}
func (UnimplementedSchemaServiceServer) GetSchema(context.Context, *GetSchemaRequest) (*Schema, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSchema not implemented")
}
func (UnimplementedSchemaServiceServer) ListSchemas(context.Context, *ListSchemasRequest) (*ListSchemasResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSchemas not implemented")
}
func (UnimplementedSchemaServiceServer) ListSchemaRevisions(context.Context, *ListSchemaRevisionsRequest) (*ListSchemaRevisionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSchemaRevisions not implemented")
}
func (UnimplementedSchemaServiceServer) CommitSchema(context.Context, *CommitSchemaRequest) (*Schema, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CommitSchema not implemented")
}
func (UnimplementedSchemaServiceServer) RollbackSchema(context.Context, *RollbackSchemaRequest) (*Schema, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RollbackSchema not implemented")
}
func (UnimplementedSchemaServiceServer) DeleteSchemaRevision(context.Context, *DeleteSchemaRevisionRequest) (*Schema, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSchemaRevision not implemented")
}
func (UnimplementedSchemaServiceServer) DeleteSchema(context.Context, *DeleteSchemaRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSchema not implemented")
}
func (UnimplementedSchemaServiceServer) ValidateSchema(context.Context, *ValidateSchemaRequest) (*ValidateSchemaResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateSchema not implemented")
}
func (UnimplementedSchemaServiceServer) ValidateMessage(context.Context, *ValidateMessageRequest) (*ValidateMessageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateMessage not implemented")
}
func (UnimplementedSchemaServiceServer) mustEmbedUnimplementedSchemaServiceServer() {}
func (UnimplementedSchemaServiceServer) testEmbeddedByValue()                       {}

// UnsafeSchemaServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SchemaServiceServer will
// result in compilation errors.
type UnsafeSchemaServiceServer interface {
	mustEmbedUnimplementedSchemaServiceServer()
}

func RegisterSchemaServiceServer(s grpc.ServiceRegistrar, srv SchemaServiceServer) {
	// If the following call pancis, it indicates UnimplementedSchemaServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SchemaService_ServiceDesc, srv)
}

func _SchemaService_CreateSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).CreateSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_CreateSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).CreateSchema(ctx, req.(*CreateSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_GetSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).GetSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_GetSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).GetSchema(ctx, req.(*GetSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_ListSchemas_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSchemasRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).ListSchemas(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_ListSchemas_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).ListSchemas(ctx, req.(*ListSchemasRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_ListSchemaRevisions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSchemaRevisionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).ListSchemaRevisions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_ListSchemaRevisions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).ListSchemaRevisions(ctx, req.(*ListSchemaRevisionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_CommitSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommitSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).CommitSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_CommitSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).CommitSchema(ctx, req.(*CommitSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_RollbackSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RollbackSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).RollbackSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_RollbackSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).RollbackSchema(ctx, req.(*RollbackSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_DeleteSchemaRevision_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSchemaRevisionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).DeleteSchemaRevision(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_DeleteSchemaRevision_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).DeleteSchemaRevision(ctx, req.(*DeleteSchemaRevisionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_DeleteSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).DeleteSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_DeleteSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).DeleteSchema(ctx, req.(*DeleteSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_ValidateSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateSchemaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).ValidateSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_ValidateSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).ValidateSchema(ctx, req.(*ValidateSchemaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SchemaService_ValidateMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ValidateMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SchemaServiceServer).ValidateMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SchemaService_ValidateMessage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SchemaServiceServer).ValidateMessage(ctx, req.(*ValidateMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SchemaService_ServiceDesc is the grpc.ServiceDesc for SchemaService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SchemaService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "google.pubsub.v1.SchemaService",
	HandlerType: (*SchemaServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateSchema",
			Handler:    _SchemaService_CreateSchema_Handler,
		},
		{
			MethodName: "GetSchema",
			Handler:    _SchemaService_GetSchema_Handler,
		},
		{
			MethodName: "ListSchemas",
			Handler:    _SchemaService_ListSchemas_Handler,
		},
		{
			MethodName: "ListSchemaRevisions",
			Handler:    _SchemaService_ListSchemaRevisions_Handler,
		},
		{
			MethodName: "CommitSchema",
			Handler:    _SchemaService_CommitSchema_Handler,
		},
		{
			MethodName: "RollbackSchema",
			Handler:    _SchemaService_RollbackSchema_Handler,
		},
		{
			MethodName: "DeleteSchemaRevision",
			Handler:    _SchemaService_DeleteSchemaRevision_Handler,
		},
		{
			MethodName: "DeleteSchema",
			Handler:    _SchemaService_DeleteSchema_Handler,
		},
		{
			MethodName: "ValidateSchema",
			Handler:    _SchemaService_ValidateSchema_Handler,
		},
		{
			MethodName: "ValidateMessage",
			Handler:    _SchemaService_ValidateMessage_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "google/pubsub/v1/schema.proto",
}