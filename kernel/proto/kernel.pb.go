// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v5.28.3
// source: kernel/proto/kernel.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type BootReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name     string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	RealmStr string   `protobuf:"bytes,2,opt,name=realmStr,proto3" json:"realmStr,omitempty"`
	Args     []string `protobuf:"bytes,3,rep,name=args,proto3" json:"args,omitempty"`
	Env      []string `protobuf:"bytes,4,rep,name=env,proto3" json:"env,omitempty"`
}

func (x *BootReq) Reset() {
	*x = BootReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BootReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BootReq) ProtoMessage() {}

func (x *BootReq) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BootReq.ProtoReflect.Descriptor instead.
func (*BootReq) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{0}
}

func (x *BootReq) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *BootReq) GetRealmStr() string {
	if x != nil {
		return x.RealmStr
	}
	return ""
}

func (x *BootReq) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *BootReq) GetEnv() []string {
	if x != nil {
		return x.Env
	}
	return nil
}

type BootRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PidStr string `protobuf:"bytes,1,opt,name=pidStr,proto3" json:"pidStr,omitempty"`
}

func (x *BootRep) Reset() {
	*x = BootRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BootRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BootRep) ProtoMessage() {}

func (x *BootRep) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BootRep.ProtoReflect.Descriptor instead.
func (*BootRep) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{1}
}

func (x *BootRep) GetPidStr() string {
	if x != nil {
		return x.PidStr
	}
	return ""
}

type EvictKernelProcReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PidStr string `protobuf:"bytes,1,opt,name=PidStr,proto3" json:"PidStr,omitempty"`
}

func (x *EvictKernelProcReq) Reset() {
	*x = EvictKernelProcReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EvictKernelProcReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EvictKernelProcReq) ProtoMessage() {}

func (x *EvictKernelProcReq) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EvictKernelProcReq.ProtoReflect.Descriptor instead.
func (*EvictKernelProcReq) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{2}
}

func (x *EvictKernelProcReq) GetPidStr() string {
	if x != nil {
		return x.PidStr
	}
	return ""
}

type EvictKernelProcRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *EvictKernelProcRep) Reset() {
	*x = EvictKernelProcRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EvictKernelProcRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EvictKernelProcRep) ProtoMessage() {}

func (x *EvictKernelProcRep) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EvictKernelProcRep.ProtoReflect.Descriptor instead.
func (*EvictKernelProcRep) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{3}
}

type SetCPUSharesReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PidStr string `protobuf:"bytes,1,opt,name=PidStr,proto3" json:"PidStr,omitempty"`
	Shares int64  `protobuf:"varint,2,opt,name=Shares,proto3" json:"Shares,omitempty"`
}

func (x *SetCPUSharesReq) Reset() {
	*x = SetCPUSharesReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetCPUSharesReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetCPUSharesReq) ProtoMessage() {}

func (x *SetCPUSharesReq) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetCPUSharesReq.ProtoReflect.Descriptor instead.
func (*SetCPUSharesReq) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{4}
}

func (x *SetCPUSharesReq) GetPidStr() string {
	if x != nil {
		return x.PidStr
	}
	return ""
}

func (x *SetCPUSharesReq) GetShares() int64 {
	if x != nil {
		return x.Shares
	}
	return 0
}

type SetCPUSharesRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *SetCPUSharesRep) Reset() {
	*x = SetCPUSharesRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetCPUSharesRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetCPUSharesRep) ProtoMessage() {}

func (x *SetCPUSharesRep) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetCPUSharesRep.ProtoReflect.Descriptor instead.
func (*SetCPUSharesRep) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{5}
}

type GetKernelSrvCPUUtilReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PidStr string `protobuf:"bytes,1,opt,name=PidStr,proto3" json:"PidStr,omitempty"`
}

func (x *GetKernelSrvCPUUtilReq) Reset() {
	*x = GetKernelSrvCPUUtilReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetKernelSrvCPUUtilReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetKernelSrvCPUUtilReq) ProtoMessage() {}

func (x *GetKernelSrvCPUUtilReq) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetKernelSrvCPUUtilReq.ProtoReflect.Descriptor instead.
func (*GetKernelSrvCPUUtilReq) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{6}
}

func (x *GetKernelSrvCPUUtilReq) GetPidStr() string {
	if x != nil {
		return x.PidStr
	}
	return ""
}

type GetKernelSrvCPUUtilRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Util float64 `protobuf:"fixed64,1,opt,name=util,proto3" json:"util,omitempty"`
}

func (x *GetKernelSrvCPUUtilRep) Reset() {
	*x = GetKernelSrvCPUUtilRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetKernelSrvCPUUtilRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetKernelSrvCPUUtilRep) ProtoMessage() {}

func (x *GetKernelSrvCPUUtilRep) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetKernelSrvCPUUtilRep.ProtoReflect.Descriptor instead.
func (*GetKernelSrvCPUUtilRep) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{7}
}

func (x *GetKernelSrvCPUUtilRep) GetUtil() float64 {
	if x != nil {
		return x.Util
	}
	return 0
}

type ShutdownReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ShutdownReq) Reset() {
	*x = ShutdownReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShutdownReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShutdownReq) ProtoMessage() {}

func (x *ShutdownReq) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShutdownReq.ProtoReflect.Descriptor instead.
func (*ShutdownReq) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{8}
}

type ShutdownRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ShutdownRep) Reset() {
	*x = ShutdownRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kernel_proto_kernel_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShutdownRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShutdownRep) ProtoMessage() {}

func (x *ShutdownRep) ProtoReflect() protoreflect.Message {
	mi := &file_kernel_proto_kernel_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShutdownRep.ProtoReflect.Descriptor instead.
func (*ShutdownRep) Descriptor() ([]byte, []int) {
	return file_kernel_proto_kernel_proto_rawDescGZIP(), []int{9}
}

var File_kernel_proto_kernel_proto protoreflect.FileDescriptor

var file_kernel_proto_kernel_proto_rawDesc = []byte{
	0x0a, 0x19, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6b,
	0x65, 0x72, 0x6e, 0x65, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5f, 0x0a, 0x07, 0x42,
	0x6f, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65,
	0x61, 0x6c, 0x6d, 0x53, 0x74, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65,
	0x61, 0x6c, 0x6d, 0x53, 0x74, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x61, 0x72, 0x67, 0x73, 0x12, 0x10, 0x0a, 0x03, 0x65, 0x6e,
	0x76, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x03, 0x65, 0x6e, 0x76, 0x22, 0x21, 0x0a, 0x07,
	0x42, 0x6f, 0x6f, 0x74, 0x52, 0x65, 0x70, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x69, 0x64, 0x53, 0x74,
	0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x70, 0x69, 0x64, 0x53, 0x74, 0x72, 0x22,
	0x2c, 0x0a, 0x12, 0x45, 0x76, 0x69, 0x63, 0x74, 0x4b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x50, 0x72,
	0x6f, 0x63, 0x52, 0x65, 0x71, 0x12, 0x16, 0x0a, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72, 0x22, 0x14, 0x0a,
	0x12, 0x45, 0x76, 0x69, 0x63, 0x74, 0x4b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x50, 0x72, 0x6f, 0x63,
	0x52, 0x65, 0x70, 0x22, 0x41, 0x0a, 0x0f, 0x53, 0x65, 0x74, 0x43, 0x50, 0x55, 0x53, 0x68, 0x61,
	0x72, 0x65, 0x73, 0x52, 0x65, 0x71, 0x12, 0x16, 0x0a, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72, 0x12, 0x16,
	0x0a, 0x06, 0x53, 0x68, 0x61, 0x72, 0x65, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06,
	0x53, 0x68, 0x61, 0x72, 0x65, 0x73, 0x22, 0x11, 0x0a, 0x0f, 0x53, 0x65, 0x74, 0x43, 0x50, 0x55,
	0x53, 0x68, 0x61, 0x72, 0x65, 0x73, 0x52, 0x65, 0x70, 0x22, 0x30, 0x0a, 0x16, 0x47, 0x65, 0x74,
	0x4b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x53, 0x72, 0x76, 0x43, 0x50, 0x55, 0x55, 0x74, 0x69, 0x6c,
	0x52, 0x65, 0x71, 0x12, 0x16, 0x0a, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x50, 0x69, 0x64, 0x53, 0x74, 0x72, 0x22, 0x2c, 0x0a, 0x16, 0x47,
	0x65, 0x74, 0x4b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x53, 0x72, 0x76, 0x43, 0x50, 0x55, 0x55, 0x74,
	0x69, 0x6c, 0x52, 0x65, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x74, 0x69, 0x6c, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x04, 0x75, 0x74, 0x69, 0x6c, 0x22, 0x0d, 0x0a, 0x0b, 0x53, 0x68, 0x75,
	0x74, 0x64, 0x6f, 0x77, 0x6e, 0x52, 0x65, 0x71, 0x22, 0x0d, 0x0a, 0x0b, 0x53, 0x68, 0x75, 0x74,
	0x64, 0x6f, 0x77, 0x6e, 0x52, 0x65, 0x70, 0x42, 0x16, 0x5a, 0x14, 0x73, 0x69, 0x67, 0x6d, 0x61,
	0x6f, 0x73, 0x2f, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_kernel_proto_kernel_proto_rawDescOnce sync.Once
	file_kernel_proto_kernel_proto_rawDescData = file_kernel_proto_kernel_proto_rawDesc
)

func file_kernel_proto_kernel_proto_rawDescGZIP() []byte {
	file_kernel_proto_kernel_proto_rawDescOnce.Do(func() {
		file_kernel_proto_kernel_proto_rawDescData = protoimpl.X.CompressGZIP(file_kernel_proto_kernel_proto_rawDescData)
	})
	return file_kernel_proto_kernel_proto_rawDescData
}

var file_kernel_proto_kernel_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_kernel_proto_kernel_proto_goTypes = []interface{}{
	(*BootReq)(nil),                // 0: BootReq
	(*BootRep)(nil),                // 1: BootRep
	(*EvictKernelProcReq)(nil),     // 2: EvictKernelProcReq
	(*EvictKernelProcRep)(nil),     // 3: EvictKernelProcRep
	(*SetCPUSharesReq)(nil),        // 4: SetCPUSharesReq
	(*SetCPUSharesRep)(nil),        // 5: SetCPUSharesRep
	(*GetKernelSrvCPUUtilReq)(nil), // 6: GetKernelSrvCPUUtilReq
	(*GetKernelSrvCPUUtilRep)(nil), // 7: GetKernelSrvCPUUtilRep
	(*ShutdownReq)(nil),            // 8: ShutdownReq
	(*ShutdownRep)(nil),            // 9: ShutdownRep
}
var file_kernel_proto_kernel_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_kernel_proto_kernel_proto_init() }
func file_kernel_proto_kernel_proto_init() {
	if File_kernel_proto_kernel_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_kernel_proto_kernel_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BootReq); i {
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
		file_kernel_proto_kernel_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BootRep); i {
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
		file_kernel_proto_kernel_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EvictKernelProcReq); i {
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
		file_kernel_proto_kernel_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EvictKernelProcRep); i {
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
		file_kernel_proto_kernel_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetCPUSharesReq); i {
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
		file_kernel_proto_kernel_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetCPUSharesRep); i {
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
		file_kernel_proto_kernel_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetKernelSrvCPUUtilReq); i {
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
		file_kernel_proto_kernel_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetKernelSrvCPUUtilRep); i {
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
		file_kernel_proto_kernel_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShutdownReq); i {
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
		file_kernel_proto_kernel_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShutdownRep); i {
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
			RawDescriptor: file_kernel_proto_kernel_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_kernel_proto_kernel_proto_goTypes,
		DependencyIndexes: file_kernel_proto_kernel_proto_depIdxs,
		MessageInfos:      file_kernel_proto_kernel_proto_msgTypes,
	}.Build()
	File_kernel_proto_kernel_proto = out.File
	file_kernel_proto_kernel_proto_rawDesc = nil
	file_kernel_proto_kernel_proto_goTypes = nil
	file_kernel_proto_kernel_proto_depIdxs = nil
}
