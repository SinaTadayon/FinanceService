// Code generated by protoc-gen-go. DO NOT EDIT.
// source: data.proto

package financesrv

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
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

type SellerFinanceList struct {
	FID                  string   `protobuf:"bytes,1,opt,name=FID,proto3" json:"FID"`
	UID                  uint64   `protobuf:"varint,2,opt,name=UID,proto3" json:"UID"`
	StartDate            string   `protobuf:"bytes,3,opt,name=StartDate,proto3" json:"StartDate"`
	EndDate              string   `protobuf:"bytes,4,opt,name=EndDate,proto3" json:"EndDate"`
	PaymentStatus        string   `protobuf:"bytes,5,opt,name=PaymentStatus,proto3" json:"PaymentStatus"`
	Total                *Money   `protobuf:"bytes,6,opt,name=Total,proto3" json:"Total"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SellerFinanceList) Reset()         { *m = SellerFinanceList{} }
func (m *SellerFinanceList) String() string { return proto.CompactTextString(m) }
func (*SellerFinanceList) ProtoMessage()    {}
func (*SellerFinanceList) Descriptor() ([]byte, []int) {
	return fileDescriptor_871986018790d2fd, []int{0}
}

func (m *SellerFinanceList) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SellerFinanceList.Unmarshal(m, b)
}
func (m *SellerFinanceList) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SellerFinanceList.Marshal(b, m, deterministic)
}
func (m *SellerFinanceList) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SellerFinanceList.Merge(m, src)
}
func (m *SellerFinanceList) XXX_Size() int {
	return xxx_messageInfo_SellerFinanceList.Size(m)
}
func (m *SellerFinanceList) XXX_DiscardUnknown() {
	xxx_messageInfo_SellerFinanceList.DiscardUnknown(m)
}

var xxx_messageInfo_SellerFinanceList proto.InternalMessageInfo

func (m *SellerFinanceList) GetFID() string {
	if m != nil {
		return m.FID
	}
	return ""
}

func (m *SellerFinanceList) GetUID() uint64 {
	if m != nil {
		return m.UID
	}
	return 0
}

func (m *SellerFinanceList) GetStartDate() string {
	if m != nil {
		return m.StartDate
	}
	return ""
}

func (m *SellerFinanceList) GetEndDate() string {
	if m != nil {
		return m.EndDate
	}
	return ""
}

func (m *SellerFinanceList) GetPaymentStatus() string {
	if m != nil {
		return m.PaymentStatus
	}
	return ""
}

func (m *SellerFinanceList) GetTotal() *Money {
	if m != nil {
		return m.Total
	}
	return nil
}

type SellerFinanceListCollection struct {
	Items                []*SellerFinanceList `protobuf:"bytes,1,rep,name=items,proto3" json:"items"`
	Total                uint64               `protobuf:"varint,2,opt,name=total,proto3" json:"total"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *SellerFinanceListCollection) Reset()         { *m = SellerFinanceListCollection{} }
func (m *SellerFinanceListCollection) String() string { return proto.CompactTextString(m) }
func (*SellerFinanceListCollection) ProtoMessage()    {}
func (*SellerFinanceListCollection) Descriptor() ([]byte, []int) {
	return fileDescriptor_871986018790d2fd, []int{1}
}

func (m *SellerFinanceListCollection) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SellerFinanceListCollection.Unmarshal(m, b)
}
func (m *SellerFinanceListCollection) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SellerFinanceListCollection.Marshal(b, m, deterministic)
}
func (m *SellerFinanceListCollection) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SellerFinanceListCollection.Merge(m, src)
}
func (m *SellerFinanceListCollection) XXX_Size() int {
	return xxx_messageInfo_SellerFinanceListCollection.Size(m)
}
func (m *SellerFinanceListCollection) XXX_DiscardUnknown() {
	xxx_messageInfo_SellerFinanceListCollection.DiscardUnknown(m)
}

var xxx_messageInfo_SellerFinanceListCollection proto.InternalMessageInfo

func (m *SellerFinanceListCollection) GetItems() []*SellerFinanceList {
	if m != nil {
		return m.Items
	}
	return nil
}

func (m *SellerFinanceListCollection) GetTotal() uint64 {
	if m != nil {
		return m.Total
	}
	return 0
}

func init() {
	proto.RegisterType((*SellerFinanceList)(nil), "financesrv.SellerFinanceList")
	proto.RegisterType((*SellerFinanceListCollection)(nil), "financesrv.SellerFinanceListCollection")
}

func init() {
	proto.RegisterFile("data.proto", fileDescriptor_871986018790d2fd)
}

var fileDescriptor_871986018790d2fd = []byte{
	// 239 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x90, 0xdb, 0x4a, 0xc3, 0x40,
	0x10, 0x86, 0x59, 0xd3, 0x54, 0x3a, 0xa5, 0x60, 0x07, 0x2f, 0x16, 0x0f, 0x10, 0x8a, 0x60, 0xae,
	0x72, 0xd1, 0x3e, 0x82, 0xb1, 0x50, 0x50, 0x90, 0x44, 0x1f, 0x60, 0x6c, 0x47, 0x0c, 0x6c, 0x77,
	0x65, 0x77, 0x14, 0xfa, 0x72, 0x3e, 0x9b, 0x64, 0x57, 0xa9, 0xa5, 0x77, 0xff, 0x7c, 0xdf, 0xec,
	0x61, 0x06, 0x60, 0x43, 0x42, 0xd5, 0x87, 0x77, 0xe2, 0x10, 0xde, 0x3a, 0x4b, 0x76, 0xcd, 0xc1,
	0x7f, 0x5d, 0x4c, 0x7e, 0x73, 0x52, 0xb3, 0x6f, 0x05, 0xd3, 0x96, 0x8d, 0x61, 0xbf, 0x4c, 0xfc,
	0xa1, 0x0b, 0x82, 0x67, 0x90, 0x2d, 0x57, 0xb5, 0x56, 0x85, 0x2a, 0x47, 0x4d, 0x1f, 0x7b, 0xf2,
	0xb2, 0xaa, 0xf5, 0x49, 0xa1, 0xca, 0x41, 0xd3, 0x47, 0xbc, 0x82, 0x51, 0x2b, 0xe4, 0xa5, 0x26,
	0x61, 0x9d, 0xc5, 0xce, 0x3d, 0x40, 0x0d, 0xa7, 0xf7, 0x76, 0x13, 0xdd, 0x20, 0xba, 0xbf, 0x12,
	0x6f, 0x60, 0xf2, 0x44, 0xbb, 0x2d, 0x5b, 0x69, 0x85, 0xe4, 0x33, 0xe8, 0x3c, 0xfa, 0x43, 0x88,
	0xb7, 0x90, 0x3f, 0x3b, 0x21, 0xa3, 0x87, 0x85, 0x2a, 0xc7, 0xf3, 0x69, 0xb5, 0x1f, 0xa1, 0x7a,
	0x74, 0x96, 0x77, 0x4d, 0xf2, 0xb3, 0x77, 0xb8, 0x3c, 0xfa, 0xff, 0x9d, 0x33, 0x86, 0xd7, 0xd2,
	0x39, 0x8b, 0x0b, 0xc8, 0x3b, 0xe1, 0x6d, 0xd0, 0xaa, 0xc8, 0xca, 0xf1, 0xfc, 0xfa, 0xff, 0x3d,
	0x47, 0xe7, 0x9a, 0xd4, 0x8b, 0xe7, 0x90, 0x4b, 0x7c, 0x3c, 0x8d, 0x9b, 0x8a, 0xd7, 0x61, 0xdc,
	0xd8, 0xe2, 0x27, 0x00, 0x00, 0xff, 0xff, 0xc6, 0xe2, 0xab, 0x55, 0x5a, 0x01, 0x00, 0x00,
}
