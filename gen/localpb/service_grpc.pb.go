// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: localpb/service.proto

package localpb

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

// ClusteringClient is the client API for Clustering service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ClusteringClient interface {
	// Ping method contacts a node based on partition ID
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	// Stats returns node stats
	Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsResponse, error)
	// Send forwards messages to nodes based on a partition ID
	Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error)
}

type clusteringClient struct {
	cc grpc.ClientConnInterface
}

func NewClusteringClient(cc grpc.ClientConnInterface) ClusteringClient {
	return &clusteringClient{cc}
}

func (c *clusteringClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, "/local.Clustering/Ping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusteringClient) Stats(ctx context.Context, in *StatsRequest, opts ...grpc.CallOption) (*StatsResponse, error) {
	out := new(StatsResponse)
	err := c.cc.Invoke(ctx, "/local.Clustering/Stats", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *clusteringClient) Send(ctx context.Context, in *SendRequest, opts ...grpc.CallOption) (*SendResponse, error) {
	out := new(SendResponse)
	err := c.cc.Invoke(ctx, "/local.Clustering/Send", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClusteringServer is the server API for Clustering service.
// All implementations should embed UnimplementedClusteringServer
// for forward compatibility
type ClusteringServer interface {
	// Ping method contacts a node based on partition ID
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	// Stats returns node stats
	Stats(context.Context, *StatsRequest) (*StatsResponse, error)
	// Send forwards messages to nodes based on a partition ID
	Send(context.Context, *SendRequest) (*SendResponse, error)
}

// UnimplementedClusteringServer should be embedded to have forward compatible implementations.
type UnimplementedClusteringServer struct {
}

func (UnimplementedClusteringServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedClusteringServer) Stats(context.Context, *StatsRequest) (*StatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stats not implemented")
}
func (UnimplementedClusteringServer) Send(context.Context, *SendRequest) (*SendResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Send not implemented")
}

// UnsafeClusteringServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ClusteringServer will
// result in compilation errors.
type UnsafeClusteringServer interface {
	mustEmbedUnimplementedClusteringServer()
}

func RegisterClusteringServer(s grpc.ServiceRegistrar, srv ClusteringServer) {
	s.RegisterService(&Clustering_ServiceDesc, srv)
}

func _Clustering_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusteringServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/local.Clustering/Ping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusteringServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Clustering_Stats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusteringServer).Stats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/local.Clustering/Stats",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusteringServer).Stats(ctx, req.(*StatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Clustering_Send_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ClusteringServer).Send(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/local.Clustering/Send",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ClusteringServer).Send(ctx, req.(*SendRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Clustering_ServiceDesc is the grpc.ServiceDesc for Clustering service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Clustering_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "local.Clustering",
	HandlerType: (*ClusteringServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _Clustering_Ping_Handler,
		},
		{
			MethodName: "Stats",
			Handler:    _Clustering_Stats_Handler,
		},
		{
			MethodName: "Send",
			Handler:    _Clustering_Send_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "localpb/service.proto",
}

// RaftClient is the client API for Raft service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RaftClient interface {
	// GetPeerDetails returns the node ID and discovery port
	GetPeerDetails(ctx context.Context, in *GetPeerDetailsRequest, opts ...grpc.CallOption) (*GetPeerDetailsResponse, error)
	// ApplyLog applies the provided request on the server if it is the leader
	ApplyLog(ctx context.Context, in *ApplyLogRequest, opts ...grpc.CallOption) (*ApplyLogResponse, error)
}

type raftClient struct {
	cc grpc.ClientConnInterface
}

func NewRaftClient(cc grpc.ClientConnInterface) RaftClient {
	return &raftClient{cc}
}

func (c *raftClient) GetPeerDetails(ctx context.Context, in *GetPeerDetailsRequest, opts ...grpc.CallOption) (*GetPeerDetailsResponse, error) {
	out := new(GetPeerDetailsResponse)
	err := c.cc.Invoke(ctx, "/local.Raft/GetPeerDetails", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *raftClient) ApplyLog(ctx context.Context, in *ApplyLogRequest, opts ...grpc.CallOption) (*ApplyLogResponse, error) {
	out := new(ApplyLogResponse)
	err := c.cc.Invoke(ctx, "/local.Raft/ApplyLog", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RaftServer is the server API for Raft service.
// All implementations should embed UnimplementedRaftServer
// for forward compatibility
type RaftServer interface {
	// GetPeerDetails returns the node ID and discovery port
	GetPeerDetails(context.Context, *GetPeerDetailsRequest) (*GetPeerDetailsResponse, error)
	// ApplyLog applies the provided request on the server if it is the leader
	ApplyLog(context.Context, *ApplyLogRequest) (*ApplyLogResponse, error)
}

// UnimplementedRaftServer should be embedded to have forward compatible implementations.
type UnimplementedRaftServer struct {
}

func (UnimplementedRaftServer) GetPeerDetails(context.Context, *GetPeerDetailsRequest) (*GetPeerDetailsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPeerDetails not implemented")
}
func (UnimplementedRaftServer) ApplyLog(context.Context, *ApplyLogRequest) (*ApplyLogResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ApplyLog not implemented")
}

// UnsafeRaftServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RaftServer will
// result in compilation errors.
type UnsafeRaftServer interface {
	mustEmbedUnimplementedRaftServer()
}

func RegisterRaftServer(s grpc.ServiceRegistrar, srv RaftServer) {
	s.RegisterService(&Raft_ServiceDesc, srv)
}

func _Raft_GetPeerDetails_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPeerDetailsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).GetPeerDetails(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/local.Raft/GetPeerDetails",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).GetPeerDetails(ctx, req.(*GetPeerDetailsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Raft_ApplyLog_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ApplyLogRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).ApplyLog(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/local.Raft/ApplyLog",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).ApplyLog(ctx, req.(*ApplyLogRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Raft_ServiceDesc is the grpc.ServiceDesc for Raft service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Raft_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "local.Raft",
	HandlerType: (*RaftServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetPeerDetails",
			Handler:    _Raft_GetPeerDetails_Handler,
		},
		{
			MethodName: "ApplyLog",
			Handler:    _Raft_ApplyLog_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "localpb/service.proto",
}
