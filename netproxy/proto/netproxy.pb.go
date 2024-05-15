// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: netproxy/proto/netproxy.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proto "sigmaos/rpc/proto"
	sigmap "sigmaos/sigmap"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ListenRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addr *sigmap.Taddr `protobuf:"bytes,1,opt,name=addr,proto3" json:"addr,omitempty"`
	Blob *proto.Blob   `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *ListenRequest) Reset() {
	*x = ListenRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListenRequest) ProtoMessage() {}

func (x *ListenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListenRequest.ProtoReflect.Descriptor instead.
func (*ListenRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{0}
}

func (x *ListenRequest) GetAddr() *sigmap.Taddr {
	if x != nil {
		return x.Addr
	}
	return nil
}

func (x *ListenRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type ListenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err        *sigmap.Rerror         `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Endpoint   *sigmap.TendpointProto `protobuf:"bytes,2,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	ListenerID uint64                 `protobuf:"varint,3,opt,name=listenerID,proto3" json:"listenerID,omitempty"`
	Blob       *proto.Blob            `protobuf:"bytes,4,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *ListenResponse) Reset() {
	*x = ListenResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListenResponse) ProtoMessage() {}

func (x *ListenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListenResponse.ProtoReflect.Descriptor instead.
func (*ListenResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{1}
}

func (x *ListenResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *ListenResponse) GetEndpoint() *sigmap.TendpointProto {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *ListenResponse) GetListenerID() uint64 {
	if x != nil {
		return x.ListenerID
	}
	return 0
}

func (x *ListenResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type DialRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Endpoint *sigmap.TendpointProto `protobuf:"bytes,1,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	Blob     *proto.Blob            `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *DialRequest) Reset() {
	*x = DialRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DialRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DialRequest) ProtoMessage() {}

func (x *DialRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DialRequest.ProtoReflect.Descriptor instead.
func (*DialRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{2}
}

func (x *DialRequest) GetEndpoint() *sigmap.TendpointProto {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *DialRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type DialResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err  *sigmap.Rerror `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Blob *proto.Blob    `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *DialResponse) Reset() {
	*x = DialResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DialResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DialResponse) ProtoMessage() {}

func (x *DialResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DialResponse.ProtoReflect.Descriptor instead.
func (*DialResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{3}
}

func (x *DialResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *DialResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type AcceptRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ListenerID uint64      `protobuf:"varint,1,opt,name=listenerID,proto3" json:"listenerID,omitempty"`
	Blob       *proto.Blob `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *AcceptRequest) Reset() {
	*x = AcceptRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AcceptRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AcceptRequest) ProtoMessage() {}

func (x *AcceptRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AcceptRequest.ProtoReflect.Descriptor instead.
func (*AcceptRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{4}
}

func (x *AcceptRequest) GetListenerID() uint64 {
	if x != nil {
		return x.ListenerID
	}
	return 0
}

func (x *AcceptRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type AcceptResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err  *sigmap.Rerror `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Blob *proto.Blob    `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *AcceptResponse) Reset() {
	*x = AcceptResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AcceptResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AcceptResponse) ProtoMessage() {}

func (x *AcceptResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AcceptResponse.ProtoReflect.Descriptor instead.
func (*AcceptResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{5}
}

func (x *AcceptResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *AcceptResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type CloseRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ListenerID uint64      `protobuf:"varint,1,opt,name=listenerID,proto3" json:"listenerID,omitempty"`
	Blob       *proto.Blob `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *CloseRequest) Reset() {
	*x = CloseRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloseRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloseRequest) ProtoMessage() {}

func (x *CloseRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloseRequest.ProtoReflect.Descriptor instead.
func (*CloseRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{6}
}

func (x *CloseRequest) GetListenerID() uint64 {
	if x != nil {
		return x.ListenerID
	}
	return 0
}

func (x *CloseRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type CloseResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err  *sigmap.Rerror `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Blob *proto.Blob    `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *CloseResponse) Reset() {
	*x = CloseResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloseResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloseResponse) ProtoMessage() {}

func (x *CloseResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloseResponse.ProtoReflect.Descriptor instead.
func (*CloseResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{7}
}

func (x *CloseResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *CloseResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type NamedEndpointRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RealmStr string      `protobuf:"bytes,1,opt,name=realmStr,proto3" json:"realmStr,omitempty"`
	Blob     *proto.Blob `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *NamedEndpointRequest) Reset() {
	*x = NamedEndpointRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NamedEndpointRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NamedEndpointRequest) ProtoMessage() {}

func (x *NamedEndpointRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NamedEndpointRequest.ProtoReflect.Descriptor instead.
func (*NamedEndpointRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{8}
}

func (x *NamedEndpointRequest) GetRealmStr() string {
	if x != nil {
		return x.RealmStr
	}
	return ""
}

func (x *NamedEndpointRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type NamedEndpointResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err      *sigmap.Rerror         `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Endpoint *sigmap.TendpointProto `protobuf:"bytes,2,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	Blob     *proto.Blob            `protobuf:"bytes,3,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *NamedEndpointResponse) Reset() {
	*x = NamedEndpointResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NamedEndpointResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NamedEndpointResponse) ProtoMessage() {}

func (x *NamedEndpointResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NamedEndpointResponse.ProtoReflect.Descriptor instead.
func (*NamedEndpointResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{9}
}

func (x *NamedEndpointResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *NamedEndpointResponse) GetEndpoint() *sigmap.TendpointProto {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *NamedEndpointResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type InvalidateNamedEndpointRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RealmStr string      `protobuf:"bytes,1,opt,name=realmStr,proto3" json:"realmStr,omitempty"`
	Blob     *proto.Blob `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *InvalidateNamedEndpointRequest) Reset() {
	*x = InvalidateNamedEndpointRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InvalidateNamedEndpointRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InvalidateNamedEndpointRequest) ProtoMessage() {}

func (x *InvalidateNamedEndpointRequest) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InvalidateNamedEndpointRequest.ProtoReflect.Descriptor instead.
func (*InvalidateNamedEndpointRequest) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{10}
}

func (x *InvalidateNamedEndpointRequest) GetRealmStr() string {
	if x != nil {
		return x.RealmStr
	}
	return ""
}

func (x *InvalidateNamedEndpointRequest) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

type InvalidateNamedEndpointResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err  *sigmap.Rerror `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
	Blob *proto.Blob    `protobuf:"bytes,2,opt,name=blob,proto3" json:"blob,omitempty"`
}

func (x *InvalidateNamedEndpointResponse) Reset() {
	*x = InvalidateNamedEndpointResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_netproxy_proto_netproxy_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InvalidateNamedEndpointResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InvalidateNamedEndpointResponse) ProtoMessage() {}

func (x *InvalidateNamedEndpointResponse) ProtoReflect() protoreflect.Message {
	mi := &file_netproxy_proto_netproxy_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InvalidateNamedEndpointResponse.ProtoReflect.Descriptor instead.
func (*InvalidateNamedEndpointResponse) Descriptor() ([]byte, []int) {
	return file_netproxy_proto_netproxy_proto_rawDescGZIP(), []int{11}
}

func (x *InvalidateNamedEndpointResponse) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

func (x *InvalidateNamedEndpointResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

var File_netproxy_proto_netproxy_proto protoreflect.FileDescriptor

var file_netproxy_proto_netproxy_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x6e, 0x65, 0x74, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x6e, 0x65, 0x74, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x13, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2f, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x13, 0x72, 0x70, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x46, 0x0a, 0x0d, 0x4c, 0x69, 0x73,
	0x74, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x04, 0x61, 0x64,
	0x64, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x06, 0x2e, 0x54, 0x61, 0x64, 0x64, 0x72,
	0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f,
	0x62, 0x22, 0x93, 0x01, 0x0a, 0x0e, 0x4c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12,
	0x2b, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x0f, 0x2e, 0x54, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x52, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x1e, 0x0a, 0x0a,
	0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x0a, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x49, 0x44, 0x12, 0x19, 0x0a, 0x04,
	0x62, 0x6c, 0x6f, 0x62, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f,
	0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x55, 0x0a, 0x0b, 0x44, 0x69, 0x61, 0x6c, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2b, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69,
	0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x54, 0x65, 0x6e, 0x64, 0x70,
	0x6f, 0x69, 0x6e, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f,
	0x69, 0x6e, 0x74, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x44,
	0x0a, 0x0c, 0x44, 0x69, 0x61, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19,
	0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x52, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f,
	0x62, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04,
	0x62, 0x6c, 0x6f, 0x62, 0x22, 0x4a, 0x0a, 0x0d, 0x41, 0x63, 0x63, 0x65, 0x70, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65,
	0x72, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0a, 0x6c, 0x69, 0x73, 0x74, 0x65,
	0x6e, 0x65, 0x72, 0x49, 0x44, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62,
	0x22, 0x46, 0x0a, 0x0e, 0x41, 0x63, 0x63, 0x65, 0x70, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12, 0x19, 0x0a,
	0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c,
	0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x49, 0x0a, 0x0c, 0x43, 0x6c, 0x6f, 0x73,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x6c, 0x69, 0x73, 0x74,
	0x65, 0x6e, 0x65, 0x72, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0a, 0x6c, 0x69,
	0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x49, 0x44, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62,
	0x6c, 0x6f, 0x62, 0x22, 0x45, 0x0a, 0x0d, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12,
	0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e,
	0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x4d, 0x0a, 0x14, 0x4e, 0x61,
	0x6d, 0x65, 0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x53, 0x74, 0x72, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x53, 0x74, 0x72, 0x12, 0x19,
	0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42,
	0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x7a, 0x0a, 0x15, 0x4e, 0x61, 0x6d,
	0x65, 0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12, 0x2b, 0x0a,
	0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0f, 0x2e, 0x54, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f,
	0x52, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c,
	0x6f, 0x62, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52,
	0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x57, 0x0a, 0x1e, 0x49, 0x6e, 0x76, 0x61, 0x6c, 0x69, 0x64,
	0x61, 0x74, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x61, 0x6c, 0x6d,
	0x53, 0x74, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x61, 0x6c, 0x6d,
	0x53, 0x74, 0x72, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x22, 0x57,
	0x0a, 0x1f, 0x49, 0x6e, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x4e, 0x61, 0x6d, 0x65,
	0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07,
	0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x12, 0x19, 0x0a, 0x04,
	0x62, 0x6c, 0x6f, 0x62, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f,
	0x62, 0x52, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x42, 0x18, 0x5a, 0x16, 0x73, 0x69, 0x67, 0x6d, 0x61,
	0x6f, 0x73, 0x2f, 0x6e, 0x65, 0x74, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_netproxy_proto_netproxy_proto_rawDescOnce sync.Once
	file_netproxy_proto_netproxy_proto_rawDescData = file_netproxy_proto_netproxy_proto_rawDesc
)

func file_netproxy_proto_netproxy_proto_rawDescGZIP() []byte {
	file_netproxy_proto_netproxy_proto_rawDescOnce.Do(func() {
		file_netproxy_proto_netproxy_proto_rawDescData = protoimpl.X.CompressGZIP(file_netproxy_proto_netproxy_proto_rawDescData)
	})
	return file_netproxy_proto_netproxy_proto_rawDescData
}

var file_netproxy_proto_netproxy_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_netproxy_proto_netproxy_proto_goTypes = []interface{}{
	(*ListenRequest)(nil),                   // 0: ListenRequest
	(*ListenResponse)(nil),                  // 1: ListenResponse
	(*DialRequest)(nil),                     // 2: DialRequest
	(*DialResponse)(nil),                    // 3: DialResponse
	(*AcceptRequest)(nil),                   // 4: AcceptRequest
	(*AcceptResponse)(nil),                  // 5: AcceptResponse
	(*CloseRequest)(nil),                    // 6: CloseRequest
	(*CloseResponse)(nil),                   // 7: CloseResponse
	(*NamedEndpointRequest)(nil),            // 8: NamedEndpointRequest
	(*NamedEndpointResponse)(nil),           // 9: NamedEndpointResponse
	(*InvalidateNamedEndpointRequest)(nil),  // 10: InvalidateNamedEndpointRequest
	(*InvalidateNamedEndpointResponse)(nil), // 11: InvalidateNamedEndpointResponse
	(*sigmap.Taddr)(nil),                    // 12: Taddr
	(*proto.Blob)(nil),                      // 13: Blob
	(*sigmap.Rerror)(nil),                   // 14: Rerror
	(*sigmap.TendpointProto)(nil),           // 15: TendpointProto
}
var file_netproxy_proto_netproxy_proto_depIdxs = []int32{
	12, // 0: ListenRequest.addr:type_name -> Taddr
	13, // 1: ListenRequest.blob:type_name -> Blob
	14, // 2: ListenResponse.err:type_name -> Rerror
	15, // 3: ListenResponse.endpoint:type_name -> TendpointProto
	13, // 4: ListenResponse.blob:type_name -> Blob
	15, // 5: DialRequest.endpoint:type_name -> TendpointProto
	13, // 6: DialRequest.blob:type_name -> Blob
	14, // 7: DialResponse.err:type_name -> Rerror
	13, // 8: DialResponse.blob:type_name -> Blob
	13, // 9: AcceptRequest.blob:type_name -> Blob
	14, // 10: AcceptResponse.err:type_name -> Rerror
	13, // 11: AcceptResponse.blob:type_name -> Blob
	13, // 12: CloseRequest.blob:type_name -> Blob
	14, // 13: CloseResponse.err:type_name -> Rerror
	13, // 14: CloseResponse.blob:type_name -> Blob
	13, // 15: NamedEndpointRequest.blob:type_name -> Blob
	14, // 16: NamedEndpointResponse.err:type_name -> Rerror
	15, // 17: NamedEndpointResponse.endpoint:type_name -> TendpointProto
	13, // 18: NamedEndpointResponse.blob:type_name -> Blob
	13, // 19: InvalidateNamedEndpointRequest.blob:type_name -> Blob
	14, // 20: InvalidateNamedEndpointResponse.err:type_name -> Rerror
	13, // 21: InvalidateNamedEndpointResponse.blob:type_name -> Blob
	22, // [22:22] is the sub-list for method output_type
	22, // [22:22] is the sub-list for method input_type
	22, // [22:22] is the sub-list for extension type_name
	22, // [22:22] is the sub-list for extension extendee
	0,  // [0:22] is the sub-list for field type_name
}

func init() { file_netproxy_proto_netproxy_proto_init() }
func file_netproxy_proto_netproxy_proto_init() {
	if File_netproxy_proto_netproxy_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_netproxy_proto_netproxy_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListenRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListenResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DialRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DialResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AcceptRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AcceptResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloseRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloseResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NamedEndpointRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NamedEndpointResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InvalidateNamedEndpointRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_netproxy_proto_netproxy_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InvalidateNamedEndpointResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_netproxy_proto_netproxy_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_netproxy_proto_netproxy_proto_goTypes,
		DependencyIndexes: file_netproxy_proto_netproxy_proto_depIdxs,
		MessageInfos:      file_netproxy_proto_netproxy_proto_msgTypes,
	}.Build()
	File_netproxy_proto_netproxy_proto = out.File
	file_netproxy_proto_netproxy_proto_rawDesc = nil
	file_netproxy_proto_netproxy_proto_goTypes = nil
	file_netproxy_proto_netproxy_proto_depIdxs = nil
}
