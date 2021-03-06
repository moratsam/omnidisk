// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package api

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ApiClient is the client API for Api service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ApiClient interface {
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	SetOffer(ctx context.Context, in *SetOfferRequest, opts ...grpc.CallOption) (*SetOfferResponse, error)
	FindCustodians(ctx context.Context, in *FindCustodiansRequest, opts ...grpc.CallOption) (*FindCustodiansResponse, error)
	RetrieveData(ctx context.Context, in *RetrieveDataRequest, opts ...grpc.CallOption) (*RetrieveDataResponse, error)
	SubscribeToEvents(ctx context.Context, in *SubscribeToEventsRequest, opts ...grpc.CallOption) (Api_SubscribeToEventsClient, error)
}

type apiClient struct {
	cc grpc.ClientConnInterface
}

func NewApiClient(cc grpc.ClientConnInterface) ApiClient {
	return &apiClient{cc}
}

func (c *apiClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, "/api.Api/Ping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) SetOffer(ctx context.Context, in *SetOfferRequest, opts ...grpc.CallOption) (*SetOfferResponse, error) {
	out := new(SetOfferResponse)
	err := c.cc.Invoke(ctx, "/api.Api/SetOffer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) FindCustodians(ctx context.Context, in *FindCustodiansRequest, opts ...grpc.CallOption) (*FindCustodiansResponse, error) {
	out := new(FindCustodiansResponse)
	err := c.cc.Invoke(ctx, "/api.Api/FindCustodians", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) RetrieveData(ctx context.Context, in *RetrieveDataRequest, opts ...grpc.CallOption) (*RetrieveDataResponse, error) {
	out := new(RetrieveDataResponse)
	err := c.cc.Invoke(ctx, "/api.Api/RetrieveData", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) SubscribeToEvents(ctx context.Context, in *SubscribeToEventsRequest, opts ...grpc.CallOption) (Api_SubscribeToEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &Api_ServiceDesc.Streams[0], "/api.Api/SubscribeToEvents", opts...)
	if err != nil {
		return nil, err
	}
	x := &apiSubscribeToEventsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Api_SubscribeToEventsClient interface {
	Recv() (*Event, error)
	grpc.ClientStream
}

type apiSubscribeToEventsClient struct {
	grpc.ClientStream
}

func (x *apiSubscribeToEventsClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ApiServer is the server API for Api service.
// All implementations must embed UnimplementedApiServer
// for forward compatibility
type ApiServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	SetOffer(context.Context, *SetOfferRequest) (*SetOfferResponse, error)
	FindCustodians(context.Context, *FindCustodiansRequest) (*FindCustodiansResponse, error)
	RetrieveData(context.Context, *RetrieveDataRequest) (*RetrieveDataResponse, error)
	SubscribeToEvents(*SubscribeToEventsRequest, Api_SubscribeToEventsServer) error
	mustEmbedUnimplementedApiServer()
}

// UnimplementedApiServer must be embedded to have forward compatible implementations.
type UnimplementedApiServer struct {
}

func (UnimplementedApiServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedApiServer) SetOffer(context.Context, *SetOfferRequest) (*SetOfferResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetOffer not implemented")
}
func (UnimplementedApiServer) FindCustodians(context.Context, *FindCustodiansRequest) (*FindCustodiansResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindCustodians not implemented")
}
func (UnimplementedApiServer) RetrieveData(context.Context, *RetrieveDataRequest) (*RetrieveDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RetrieveData not implemented")
}
func (UnimplementedApiServer) SubscribeToEvents(*SubscribeToEventsRequest, Api_SubscribeToEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeToEvents not implemented")
}
func (UnimplementedApiServer) mustEmbedUnimplementedApiServer() {}

// UnsafeApiServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ApiServer will
// result in compilation errors.
type UnsafeApiServer interface {
	mustEmbedUnimplementedApiServer()
}

func RegisterApiServer(s grpc.ServiceRegistrar, srv ApiServer) {
	s.RegisterService(&Api_ServiceDesc, srv)
}

func _Api_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Api/Ping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Api_SetOffer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetOfferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).SetOffer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Api/SetOffer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).SetOffer(ctx, req.(*SetOfferRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Api_FindCustodians_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FindCustodiansRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).FindCustodians(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Api/FindCustodians",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).FindCustodians(ctx, req.(*FindCustodiansRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Api_RetrieveData_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RetrieveDataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiServer).RetrieveData(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Api/RetrieveData",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiServer).RetrieveData(ctx, req.(*RetrieveDataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Api_SubscribeToEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeToEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ApiServer).SubscribeToEvents(m, &apiSubscribeToEventsServer{stream})
}

type Api_SubscribeToEventsServer interface {
	Send(*Event) error
	grpc.ServerStream
}

type apiSubscribeToEventsServer struct {
	grpc.ServerStream
}

func (x *apiSubscribeToEventsServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

// Api_ServiceDesc is the grpc.ServiceDesc for Api service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Api_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "api.Api",
	HandlerType: (*ApiServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _Api_Ping_Handler,
		},
		{
			MethodName: "SetOffer",
			Handler:    _Api_SetOffer_Handler,
		},
		{
			MethodName: "FindCustodians",
			Handler:    _Api_FindCustodians_Handler,
		},
		{
			MethodName: "RetrieveData",
			Handler:    _Api_RetrieveData_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SubscribeToEvents",
			Handler:       _Api_SubscribeToEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "api.proto",
}
