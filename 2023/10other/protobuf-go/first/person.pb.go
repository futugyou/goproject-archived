// Code generated by protoc-gen-go. DO NOT EDIT.
// source: first/person.proto

package first

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
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

type PersonMessage struct {
	Id                   int32    `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Is_Adult             bool     `protobuf:"varint,2,opt,name=is_Adult,json=isAdult,proto3" json:"is_Adult,omitempty"`
	Name                 string   `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	LuckyNumbers         []int32  `protobuf:"varint,4,rep,packed,name=lucky_numbers,json=luckyNumbers,proto3" json:"lucky_numbers,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PersonMessage) Reset()         { *m = PersonMessage{} }
func (m *PersonMessage) String() string { return proto.CompactTextString(m) }
func (*PersonMessage) ProtoMessage()    {}
func (*PersonMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_3c72413105cc8752, []int{0}
}

func (m *PersonMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PersonMessage.Unmarshal(m, b)
}
func (m *PersonMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PersonMessage.Marshal(b, m, deterministic)
}
func (m *PersonMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PersonMessage.Merge(m, src)
}
func (m *PersonMessage) XXX_Size() int {
	return xxx_messageInfo_PersonMessage.Size(m)
}
func (m *PersonMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_PersonMessage.DiscardUnknown(m)
}

var xxx_messageInfo_PersonMessage proto.InternalMessageInfo

func (m *PersonMessage) GetId() int32 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *PersonMessage) GetIs_Adult() bool {
	if m != nil {
		return m.Is_Adult
	}
	return false
}

func (m *PersonMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *PersonMessage) GetLuckyNumbers() []int32 {
	if m != nil {
		return m.LuckyNumbers
	}
	return nil
}

func init() {
	proto.RegisterType((*PersonMessage)(nil), "person.first.PersonMessage")
}

func init() { proto.RegisterFile("first/person.proto", fileDescriptor_3c72413105cc8752) }

var fileDescriptor_3c72413105cc8752 = []byte{
	// 161 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x4a, 0xcb, 0x2c, 0x2a,
	0x2e, 0xd1, 0x2f, 0x48, 0x2d, 0x2a, 0xce, 0xcf, 0xd3, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2,
	0x81, 0xf2, 0xc0, 0x52, 0x4a, 0xc5, 0x5c, 0xbc, 0x01, 0x60, 0xbe, 0x6f, 0x6a, 0x71, 0x71, 0x62,
	0x7a, 0xaa, 0x10, 0x1f, 0x17, 0x53, 0x66, 0x8a, 0x04, 0xa3, 0x02, 0xa3, 0x06, 0x6b, 0x10, 0x53,
	0x66, 0x8a, 0x90, 0x24, 0x17, 0x47, 0x66, 0x71, 0xbc, 0x63, 0x4a, 0x69, 0x4e, 0x89, 0x04, 0x93,
	0x02, 0xa3, 0x06, 0x47, 0x10, 0x7b, 0x66, 0x31, 0x98, 0x2b, 0x24, 0xc4, 0xc5, 0x92, 0x97, 0x98,
	0x9b, 0x2a, 0xc1, 0xac, 0xc0, 0xa8, 0xc1, 0x19, 0x04, 0x66, 0x0b, 0x29, 0x73, 0xf1, 0xe6, 0x94,
	0x26, 0x67, 0x57, 0xc6, 0xe7, 0x95, 0xe6, 0x26, 0xa5, 0x16, 0x15, 0x4b, 0xb0, 0x28, 0x30, 0x6b,
	0xb0, 0x06, 0xf1, 0x80, 0x05, 0xfd, 0x20, 0x62, 0x4e, 0xec, 0x51, 0xac, 0x60, 0xdb, 0x93, 0xd8,
	0xc0, 0x4e, 0x32, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0xcd, 0xc4, 0x6e, 0xff, 0xa8, 0x00, 0x00,
	0x00,
}
