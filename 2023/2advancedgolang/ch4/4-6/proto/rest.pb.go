// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rest.proto

package rest

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type StringMessage struct {
	Value                string   `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StringMessage) Reset()         { *m = StringMessage{} }
func (m *StringMessage) String() string { return proto.CompactTextString(m) }
func (*StringMessage) ProtoMessage()    {}
func (*StringMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_8ecebe7e2bec4bc1, []int{0}
}

func (m *StringMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StringMessage.Unmarshal(m, b)
}
func (m *StringMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StringMessage.Marshal(b, m, deterministic)
}
func (m *StringMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StringMessage.Merge(m, src)
}
func (m *StringMessage) XXX_Size() int {
	return xxx_messageInfo_StringMessage.Size(m)
}
func (m *StringMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_StringMessage.DiscardUnknown(m)
}

var xxx_messageInfo_StringMessage proto.InternalMessageInfo

func (m *StringMessage) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

func init() {
	proto.RegisterType((*StringMessage)(nil), "rest.StringMessage")
}

func init() { proto.RegisterFile("rest.proto", fileDescriptor_8ecebe7e2bec4bc1) }

var fileDescriptor_8ecebe7e2bec4bc1 = []byte{
	// 183 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2a, 0x4a, 0x2d, 0x2e,
	0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x01, 0xb1, 0xa5, 0x64, 0xd2, 0xf3, 0xf3, 0xd3,
	0x73, 0x52, 0xf5, 0x13, 0x0b, 0x32, 0xf5, 0x13, 0xf3, 0xf2, 0xf2, 0x4b, 0x12, 0x4b, 0x32, 0xf3,
	0xf3, 0x8a, 0x21, 0x6a, 0x94, 0x54, 0xb9, 0x78, 0x83, 0x4b, 0x8a, 0x32, 0xf3, 0xd2, 0x7d, 0x53,
	0x8b, 0x8b, 0x13, 0xd3, 0x53, 0x85, 0x44, 0xb8, 0x58, 0xcb, 0x12, 0x73, 0x4a, 0x53, 0x25, 0x18,
	0x15, 0x18, 0x35, 0x38, 0x83, 0x20, 0x1c, 0xa3, 0x19, 0x8c, 0x5c, 0xdc, 0x41, 0xa9, 0xc5, 0x25,
	0xc1, 0xa9, 0x45, 0x65, 0x99, 0xc9, 0xa9, 0x42, 0xae, 0x5c, 0xcc, 0xee, 0xa9, 0x25, 0x42, 0xc2,
	0x7a, 0x60, 0xeb, 0x50, 0x4c, 0x90, 0xc2, 0x26, 0xa8, 0x24, 0xd2, 0x74, 0xf9, 0xc9, 0x64, 0x26,
	0x3e, 0x21, 0x1e, 0xfd, 0xf4, 0xd4, 0x12, 0xfd, 0x6a, 0xb0, 0xa9, 0xb5, 0x42, 0x4e, 0x5c, 0x2c,
	0x01, 0xf9, 0xc5, 0xa4, 0x98, 0x23, 0x00, 0x36, 0x87, 0x4b, 0x89, 0x55, 0xbf, 0x20, 0xbf, 0xb8,
	0xc4, 0x8a, 0x51, 0x2b, 0x89, 0x0d, 0xec, 0x11, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0xa4,
	0xba, 0x6f, 0x1c, 0xfa, 0x00, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RestServiceClient is the client API for RestService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RestServiceClient interface {
	Get(ctx context.Context, in *StringMessage, opts ...grpc.CallOption) (*StringMessage, error)
	Post(ctx context.Context, in *StringMessage, opts ...grpc.CallOption) (*StringMessage, error)
}

type restServiceClient struct {
	cc *grpc.ClientConn
}

func NewRestServiceClient(cc *grpc.ClientConn) RestServiceClient {
	return &restServiceClient{cc}
}

func (c *restServiceClient) Get(ctx context.Context, in *StringMessage, opts ...grpc.CallOption) (*StringMessage, error) {
	out := new(StringMessage)
	err := c.cc.Invoke(ctx, "/rest.RestService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restServiceClient) Post(ctx context.Context, in *StringMessage, opts ...grpc.CallOption) (*StringMessage, error) {
	out := new(StringMessage)
	err := c.cc.Invoke(ctx, "/rest.RestService/Post", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RestServiceServer is the server API for RestService service.
type RestServiceServer interface {
	Get(context.Context, *StringMessage) (*StringMessage, error)
	Post(context.Context, *StringMessage) (*StringMessage, error)
}

// UnimplementedRestServiceServer can be embedded to have forward compatible implementations.
type UnimplementedRestServiceServer struct {
}

func (*UnimplementedRestServiceServer) Get(ctx context.Context, req *StringMessage) (*StringMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (*UnimplementedRestServiceServer) Post(ctx context.Context, req *StringMessage) (*StringMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Post not implemented")
}

func RegisterRestServiceServer(s *grpc.Server, srv RestServiceServer) {
	s.RegisterService(&_RestService_serviceDesc, srv)
}

func _RestService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StringMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RestServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rest.RestService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RestServiceServer).Get(ctx, req.(*StringMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _RestService_Post_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StringMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RestServiceServer).Post(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/rest.RestService/Post",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RestServiceServer).Post(ctx, req.(*StringMessage))
	}
	return interceptor(ctx, in, info, handler)
}

var _RestService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "rest.RestService",
	HandlerType: (*RestServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Get",
			Handler:    _RestService_Get_Handler,
		},
		{
			MethodName: "Post",
			Handler:    _RestService_Post_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "rest.proto",
}