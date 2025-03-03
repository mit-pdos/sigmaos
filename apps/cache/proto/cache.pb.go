// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: apps/cache/proto/cache.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sigmap "sigmaos/sigmap"
	proto "sigmaos/util/tracing/proto"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CacheReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key               string                   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value             []byte                   `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	Shard             uint32                   `protobuf:"varint,3,opt,name=shard,proto3" json:"shard,omitempty"`
	Mode              uint32                   `protobuf:"varint,4,opt,name=mode,proto3" json:"mode,omitempty"`
	SpanContextConfig *proto.SpanContextConfig `protobuf:"bytes,5,opt,name=spanContextConfig,proto3" json:"spanContextConfig,omitempty"`
	Fence             *sigmap.TfenceProto      `protobuf:"bytes,6,opt,name=fence,proto3" json:"fence,omitempty"`
}

func (x *CacheReq) Reset() {
	*x = CacheReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheReq) ProtoMessage() {}

func (x *CacheReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheReq.ProtoReflect.Descriptor instead.
func (*CacheReq) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{0}
}

func (x *CacheReq) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *CacheReq) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

func (x *CacheReq) GetShard() uint32 {
	if x != nil {
		return x.Shard
	}
	return 0
}

func (x *CacheReq) GetMode() uint32 {
	if x != nil {
		return x.Mode
	}
	return 0
}

func (x *CacheReq) GetSpanContextConfig() *proto.SpanContextConfig {
	if x != nil {
		return x.SpanContextConfig
	}
	return nil
}

func (x *CacheReq) GetFence() *sigmap.TfenceProto {
	if x != nil {
		return x.Fence
	}
	return nil
}

type ShardReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Shard uint32              `protobuf:"varint,1,opt,name=shard,proto3" json:"shard,omitempty"`
	Fence *sigmap.TfenceProto `protobuf:"bytes,2,opt,name=fence,proto3" json:"fence,omitempty"`
	Vals  map[string][]byte   `protobuf:"bytes,3,rep,name=vals,proto3" json:"vals,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *ShardReq) Reset() {
	*x = ShardReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShardReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShardReq) ProtoMessage() {}

func (x *ShardReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShardReq.ProtoReflect.Descriptor instead.
func (*ShardReq) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{1}
}

func (x *ShardReq) GetShard() uint32 {
	if x != nil {
		return x.Shard
	}
	return 0
}

func (x *ShardReq) GetFence() *sigmap.TfenceProto {
	if x != nil {
		return x.Fence
	}
	return nil
}

func (x *ShardReq) GetVals() map[string][]byte {
	if x != nil {
		return x.Vals
	}
	return nil
}

type CacheOK struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CacheOK) Reset() {
	*x = CacheOK{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheOK) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheOK) ProtoMessage() {}

func (x *CacheOK) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheOK.ProtoReflect.Descriptor instead.
func (*CacheOK) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{2}
}

type CacheRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value []byte `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *CacheRep) Reset() {
	*x = CacheRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheRep) ProtoMessage() {}

func (x *CacheRep) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheRep.ProtoReflect.Descriptor instead.
func (*CacheRep) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{3}
}

func (x *CacheRep) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

type ShardData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Vals map[string][]byte `protobuf:"bytes,1,rep,name=vals,proto3" json:"vals,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *ShardData) Reset() {
	*x = ShardData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShardData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShardData) ProtoMessage() {}

func (x *ShardData) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShardData.ProtoReflect.Descriptor instead.
func (*ShardData) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{4}
}

func (x *ShardData) GetVals() map[string][]byte {
	if x != nil {
		return x.Vals
	}
	return nil
}

type CacheString struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Val string `protobuf:"bytes,1,opt,name=val,proto3" json:"val,omitempty"`
}

func (x *CacheString) Reset() {
	*x = CacheString{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheString) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheString) ProtoMessage() {}

func (x *CacheString) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheString.ProtoReflect.Descriptor instead.
func (*CacheString) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{5}
}

func (x *CacheString) GetVal() string {
	if x != nil {
		return x.Val
	}
	return ""
}

type CacheInt struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Val int64 `protobuf:"varint,1,opt,name=val,proto3" json:"val,omitempty"`
}

func (x *CacheInt) Reset() {
	*x = CacheInt{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_cache_proto_cache_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheInt) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheInt) ProtoMessage() {}

func (x *CacheInt) ProtoReflect() protoreflect.Message {
	mi := &file_apps_cache_proto_cache_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheInt.ProtoReflect.Descriptor instead.
func (*CacheInt) Descriptor() ([]byte, []int) {
	return file_apps_cache_proto_cache_proto_rawDescGZIP(), []int{6}
}

func (x *CacheInt) GetVal() int64 {
	if x != nil {
		return x.Val
	}
	return 0
}

var File_apps_cache_proto_cache_proto protoreflect.FileDescriptor

var file_apps_cache_proto_cache_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x61, 0x70, 0x70, 0x73, 0x2f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x20,
	0x75, 0x74, 0x69, 0x6c, 0x2f, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x13, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2f, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc2, 0x01, 0x0a, 0x08, 0x43, 0x61, 0x63, 0x68, 0x65, 0x52,
	0x65, 0x71, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x68,
	0x61, 0x72, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x73, 0x68, 0x61, 0x72, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04,
	0x6d, 0x6f, 0x64, 0x65, 0x12, 0x40, 0x0a, 0x11, 0x73, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74,
	0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x12, 0x2e, 0x53, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x52, 0x11, 0x73, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x22, 0x0a, 0x05, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x54, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x52, 0x05, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x22, 0xa6, 0x01, 0x0a, 0x08, 0x53,
	0x68, 0x61, 0x72, 0x64, 0x52, 0x65, 0x71, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x68, 0x61, 0x72, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x73, 0x68, 0x61, 0x72, 0x64, 0x12, 0x22, 0x0a,
	0x05, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x54,
	0x66, 0x65, 0x6e, 0x63, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x05, 0x66, 0x65, 0x6e, 0x63,
	0x65, 0x12, 0x27, 0x0a, 0x04, 0x76, 0x61, 0x6c, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x13, 0x2e, 0x53, 0x68, 0x61, 0x72, 0x64, 0x52, 0x65, 0x71, 0x2e, 0x56, 0x61, 0x6c, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x76, 0x61, 0x6c, 0x73, 0x1a, 0x37, 0x0a, 0x09, 0x56, 0x61,
	0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x22, 0x09, 0x0a, 0x07, 0x43, 0x61, 0x63, 0x68, 0x65, 0x4f, 0x4b, 0x22, 0x20,
	0x0a, 0x08, 0x43, 0x61, 0x63, 0x68, 0x65, 0x52, 0x65, 0x70, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x22, 0x6e, 0x0a, 0x09, 0x53, 0x68, 0x61, 0x72, 0x64, 0x44, 0x61, 0x74, 0x61, 0x12, 0x28, 0x0a,
	0x04, 0x76, 0x61, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x53, 0x68,
	0x61, 0x72, 0x64, 0x44, 0x61, 0x74, 0x61, 0x2e, 0x56, 0x61, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x52, 0x04, 0x76, 0x61, 0x6c, 0x73, 0x1a, 0x37, 0x0a, 0x09, 0x56, 0x61, 0x6c, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01,
	0x22, 0x1f, 0x0a, 0x0b, 0x43, 0x61, 0x63, 0x68, 0x65, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x12,
	0x10, 0x0a, 0x03, 0x76, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x76, 0x61,
	0x6c, 0x22, 0x1c, 0x0a, 0x08, 0x43, 0x61, 0x63, 0x68, 0x65, 0x49, 0x6e, 0x74, 0x12, 0x10, 0x0a,
	0x03, 0x76, 0x61, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x03, 0x76, 0x61, 0x6c, 0x42,
	0x1a, 0x5a, 0x18, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x61, 0x70, 0x70, 0x73, 0x2f,
	0x63, 0x61, 0x63, 0x68, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_apps_cache_proto_cache_proto_rawDescOnce sync.Once
	file_apps_cache_proto_cache_proto_rawDescData = file_apps_cache_proto_cache_proto_rawDesc
)

func file_apps_cache_proto_cache_proto_rawDescGZIP() []byte {
	file_apps_cache_proto_cache_proto_rawDescOnce.Do(func() {
		file_apps_cache_proto_cache_proto_rawDescData = protoimpl.X.CompressGZIP(file_apps_cache_proto_cache_proto_rawDescData)
	})
	return file_apps_cache_proto_cache_proto_rawDescData
}

var file_apps_cache_proto_cache_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_apps_cache_proto_cache_proto_goTypes = []interface{}{
	(*CacheReq)(nil),                // 0: CacheReq
	(*ShardReq)(nil),                // 1: ShardReq
	(*CacheOK)(nil),                 // 2: CacheOK
	(*CacheRep)(nil),                // 3: CacheRep
	(*ShardData)(nil),               // 4: ShardData
	(*CacheString)(nil),             // 5: CacheString
	(*CacheInt)(nil),                // 6: CacheInt
	nil,                             // 7: ShardReq.ValsEntry
	nil,                             // 8: ShardData.ValsEntry
	(*proto.SpanContextConfig)(nil), // 9: SpanContextConfig
	(*sigmap.TfenceProto)(nil),      // 10: TfenceProto
}
var file_apps_cache_proto_cache_proto_depIdxs = []int32{
	9,  // 0: CacheReq.spanContextConfig:type_name -> SpanContextConfig
	10, // 1: CacheReq.fence:type_name -> TfenceProto
	10, // 2: ShardReq.fence:type_name -> TfenceProto
	7,  // 3: ShardReq.vals:type_name -> ShardReq.ValsEntry
	8,  // 4: ShardData.vals:type_name -> ShardData.ValsEntry
	5,  // [5:5] is the sub-list for method output_type
	5,  // [5:5] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_apps_cache_proto_cache_proto_init() }
func file_apps_cache_proto_cache_proto_init() {
	if File_apps_cache_proto_cache_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_apps_cache_proto_cache_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheReq); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShardReq); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheOK); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheRep); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShardData); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheString); i {
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
		file_apps_cache_proto_cache_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheInt); i {
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
			RawDescriptor: file_apps_cache_proto_cache_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_apps_cache_proto_cache_proto_goTypes,
		DependencyIndexes: file_apps_cache_proto_cache_proto_depIdxs,
		MessageInfos:      file_apps_cache_proto_cache_proto_msgTypes,
	}.Build()
	File_apps_cache_proto_cache_proto = out.File
	file_apps_cache_proto_cache_proto_rawDesc = nil
	file_apps_cache_proto_cache_proto_goTypes = nil
	file_apps_cache_proto_cache_proto_depIdxs = nil
}
