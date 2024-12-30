// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: sched/msched/proc/proto/uproc.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proc "sigmaos/proc"
	sigmap "sigmaos/sigmap"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RunReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProcProto *proc.ProcProto `protobuf:"bytes,1,opt,name=procProto,proto3" json:"procProto,omitempty"`
}

func (x *RunReq) Reset() {
	*x = RunReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunReq) ProtoMessage() {}

func (x *RunReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunReq.ProtoReflect.Descriptor instead.
func (*RunReq) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{0}
}

func (x *RunReq) GetProcProto() *proc.ProcProto {
	if x != nil {
		return x.ProcProto
	}
	return nil
}

type RunRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RunRep) Reset() {
	*x = RunRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunRep) ProtoMessage() {}

func (x *RunRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunRep.ProtoReflect.Descriptor instead.
func (*RunRep) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{1}
}

type WarmBinReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RealmStr           string                 `protobuf:"bytes,1,opt,name=realmStr,proto3" json:"realmStr,omitempty"`
	Program            string                 `protobuf:"bytes,2,opt,name=program,proto3" json:"program,omitempty"`
	PidStr             string                 `protobuf:"bytes,3,opt,name=pidStr,proto3" json:"pidStr,omitempty"`
	SigmaPath          []string               `protobuf:"bytes,4,rep,name=sigmaPath,proto3" json:"sigmaPath,omitempty"`
	S3Secret           *sigmap.SecretProto    `protobuf:"bytes,5,opt,name=s3Secret,proto3" json:"s3Secret,omitempty"`
	NamedEndpointProto *sigmap.TendpointProto `protobuf:"bytes,6,opt,name=NamedEndpointProto,proto3" json:"NamedEndpointProto,omitempty"`
}

func (x *WarmBinReq) Reset() {
	*x = WarmBinReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WarmBinReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WarmBinReq) ProtoMessage() {}

func (x *WarmBinReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WarmBinReq.ProtoReflect.Descriptor instead.
func (*WarmBinReq) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{2}
}

func (x *WarmBinReq) GetRealmStr() string {
	if x != nil {
		return x.RealmStr
	}
	return ""
}

func (x *WarmBinReq) GetProgram() string {
	if x != nil {
		return x.Program
	}
	return ""
}

func (x *WarmBinReq) GetPidStr() string {
	if x != nil {
		return x.PidStr
	}
	return ""
}

func (x *WarmBinReq) GetSigmaPath() []string {
	if x != nil {
		return x.SigmaPath
	}
	return nil
}

func (x *WarmBinReq) GetS3Secret() *sigmap.SecretProto {
	if x != nil {
		return x.S3Secret
	}
	return nil
}

func (x *WarmBinReq) GetNamedEndpointProto() *sigmap.TendpointProto {
	if x != nil {
		return x.NamedEndpointProto
	}
	return nil
}

type WarmBinRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OK bool `protobuf:"varint,1,opt,name=oK,proto3" json:"oK,omitempty"`
}

func (x *WarmBinRep) Reset() {
	*x = WarmBinRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WarmBinRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WarmBinRep) ProtoMessage() {}

func (x *WarmBinRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WarmBinRep.ProtoReflect.Descriptor instead.
func (*WarmBinRep) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{3}
}

func (x *WarmBinRep) GetOK() bool {
	if x != nil {
		return x.OK
	}
	return false
}

type FetchReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Prog    string `protobuf:"bytes,1,opt,name=prog,proto3" json:"prog,omitempty"`
	ChunkId int32  `protobuf:"varint,2,opt,name=chunkId,proto3" json:"chunkId,omitempty"`
	Size    uint64 `protobuf:"varint,3,opt,name=size,proto3" json:"size,omitempty"`
	Pid     uint32 `protobuf:"varint,4,opt,name=pid,proto3" json:"pid,omitempty"`
}

func (x *FetchReq) Reset() {
	*x = FetchReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchReq) ProtoMessage() {}

func (x *FetchReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchReq.ProtoReflect.Descriptor instead.
func (*FetchReq) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{4}
}

func (x *FetchReq) GetProg() string {
	if x != nil {
		return x.Prog
	}
	return ""
}

func (x *FetchReq) GetChunkId() int32 {
	if x != nil {
		return x.ChunkId
	}
	return 0
}

func (x *FetchReq) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *FetchReq) GetPid() uint32 {
	if x != nil {
		return x.Pid
	}
	return 0
}

type FetchRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Size uint64 `protobuf:"varint,1,opt,name=size,proto3" json:"size,omitempty"`
}

func (x *FetchRep) Reset() {
	*x = FetchRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchRep) ProtoMessage() {}

func (x *FetchRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchRep.ProtoReflect.Descriptor instead.
func (*FetchRep) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{5}
}

func (x *FetchRep) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

type LookupReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Prog string `protobuf:"bytes,1,opt,name=prog,proto3" json:"prog,omitempty"`
	Pid  uint32 `protobuf:"varint,2,opt,name=pid,proto3" json:"pid,omitempty"`
}

func (x *LookupReq) Reset() {
	*x = LookupReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LookupReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LookupReq) ProtoMessage() {}

func (x *LookupReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LookupReq.ProtoReflect.Descriptor instead.
func (*LookupReq) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{6}
}

func (x *LookupReq) GetProg() string {
	if x != nil {
		return x.Prog
	}
	return ""
}

func (x *LookupReq) GetPid() uint32 {
	if x != nil {
		return x.Pid
	}
	return 0
}

type LookupRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Stat *sigmap.TstatProto `protobuf:"bytes,1,opt,name=stat,proto3" json:"stat,omitempty"`
}

func (x *LookupRep) Reset() {
	*x = LookupRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LookupRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LookupRep) ProtoMessage() {}

func (x *LookupRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_msched_proc_proto_uproc_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LookupRep.ProtoReflect.Descriptor instead.
func (*LookupRep) Descriptor() ([]byte, []int) {
	return file_sched_msched_proc_proto_uproc_proto_rawDescGZIP(), []int{7}
}

func (x *LookupRep) GetStat() *sigmap.TstatProto {
	if x != nil {
		return x.Stat
	}
	return nil
}

var File_sched_msched_proc_proto_uproc_proto protoreflect.FileDescriptor

var file_sched_msched_proc_proto_uproc_proto_rawDesc = []byte{
	0x0a, 0x23, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x6d, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x70,
	0x72, 0x6f, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x75, 0x70, 0x72, 0x6f, 0x63, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0f, 0x70, 0x72, 0x6f, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x63,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x13, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2f, 0x73,
	0x69, 0x67, 0x6d, 0x61, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x32, 0x0a, 0x06, 0x52,
	0x75, 0x6e, 0x52, 0x65, 0x71, 0x12, 0x28, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x50,
	0x72, 0x6f, 0x74, 0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x08, 0x0a, 0x06, 0x52, 0x75, 0x6e, 0x52, 0x65, 0x70, 0x22, 0xe3, 0x01, 0x0a, 0x0a, 0x57, 0x61,
	0x72, 0x6d, 0x42, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x61, 0x6c,
	0x6d, 0x53, 0x74, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x61, 0x6c,
	0x6d, 0x53, 0x74, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x61, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x61, 0x6d, 0x12, 0x16,
	0x0a, 0x06, 0x70, 0x69, 0x64, 0x53, 0x74, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x70, 0x69, 0x64, 0x53, 0x74, 0x72, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x50,
	0x61, 0x74, 0x68, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6d, 0x61,
	0x50, 0x61, 0x74, 0x68, 0x12, 0x28, 0x0a, 0x08, 0x73, 0x33, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x50,
	0x72, 0x6f, 0x74, 0x6f, 0x52, 0x08, 0x73, 0x33, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x3f,
	0x0a, 0x12, 0x4e, 0x61, 0x6d, 0x65, 0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x50,
	0x72, 0x6f, 0x74, 0x6f, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x54, 0x65, 0x6e,
	0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x12, 0x4e, 0x61, 0x6d,
	0x65, 0x64, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x1c, 0x0a, 0x0a, 0x57, 0x61, 0x72, 0x6d, 0x42, 0x69, 0x6e, 0x52, 0x65, 0x70, 0x12, 0x0e, 0x0a,
	0x02, 0x6f, 0x4b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x02, 0x6f, 0x4b, 0x22, 0x5e, 0x0a,
	0x08, 0x46, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65, 0x71, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x72, 0x6f,
	0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x72, 0x6f, 0x67, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x49, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07,
	0x63, 0x68, 0x75, 0x6e, 0x6b, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x70,
	0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x70, 0x69, 0x64, 0x22, 0x1e, 0x0a,
	0x08, 0x46, 0x65, 0x74, 0x63, 0x68, 0x52, 0x65, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x22, 0x31, 0x0a,
	0x09, 0x4c, 0x6f, 0x6f, 0x6b, 0x75, 0x70, 0x52, 0x65, 0x71, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x72,
	0x6f, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x72, 0x6f, 0x67, 0x12, 0x10,
	0x0a, 0x03, 0x70, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x70, 0x69, 0x64,
	0x22, 0x2c, 0x0a, 0x09, 0x4c, 0x6f, 0x6f, 0x6b, 0x75, 0x70, 0x52, 0x65, 0x70, 0x12, 0x1f, 0x0a,
	0x04, 0x73, 0x74, 0x61, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x54, 0x73,
	0x74, 0x61, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x04, 0x73, 0x74, 0x61, 0x74, 0x42, 0x21,
	0x5a, 0x1f, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f,
	0x6d, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sched_msched_proc_proto_uproc_proto_rawDescOnce sync.Once
	file_sched_msched_proc_proto_uproc_proto_rawDescData = file_sched_msched_proc_proto_uproc_proto_rawDesc
)

func file_sched_msched_proc_proto_uproc_proto_rawDescGZIP() []byte {
	file_sched_msched_proc_proto_uproc_proto_rawDescOnce.Do(func() {
		file_sched_msched_proc_proto_uproc_proto_rawDescData = protoimpl.X.CompressGZIP(file_sched_msched_proc_proto_uproc_proto_rawDescData)
	})
	return file_sched_msched_proc_proto_uproc_proto_rawDescData
}

var file_sched_msched_proc_proto_uproc_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_sched_msched_proc_proto_uproc_proto_goTypes = []interface{}{
	(*RunReq)(nil),                // 0: RunReq
	(*RunRep)(nil),                // 1: RunRep
	(*WarmBinReq)(nil),            // 2: WarmBinReq
	(*WarmBinRep)(nil),            // 3: WarmBinRep
	(*FetchReq)(nil),              // 4: FetchReq
	(*FetchRep)(nil),              // 5: FetchRep
	(*LookupReq)(nil),             // 6: LookupReq
	(*LookupRep)(nil),             // 7: LookupRep
	(*proc.ProcProto)(nil),        // 8: ProcProto
	(*sigmap.SecretProto)(nil),    // 9: SecretProto
	(*sigmap.TendpointProto)(nil), // 10: TendpointProto
	(*sigmap.TstatProto)(nil),     // 11: TstatProto
}
var file_sched_msched_proc_proto_uproc_proto_depIdxs = []int32{
	8,  // 0: RunReq.procProto:type_name -> ProcProto
	9,  // 1: WarmBinReq.s3Secret:type_name -> SecretProto
	10, // 2: WarmBinReq.NamedEndpointProto:type_name -> TendpointProto
	11, // 3: LookupRep.stat:type_name -> TstatProto
	4,  // [4:4] is the sub-list for method output_type
	4,  // [4:4] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_sched_msched_proc_proto_uproc_proto_init() }
func file_sched_msched_proc_proto_uproc_proto_init() {
	if File_sched_msched_proc_proto_uproc_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sched_msched_proc_proto_uproc_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunReq); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunRep); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WarmBinReq); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WarmBinRep); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchReq); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchRep); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LookupReq); i {
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
		file_sched_msched_proc_proto_uproc_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LookupRep); i {
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
			RawDescriptor: file_sched_msched_proc_proto_uproc_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sched_msched_proc_proto_uproc_proto_goTypes,
		DependencyIndexes: file_sched_msched_proc_proto_uproc_proto_depIdxs,
		MessageInfos:      file_sched_msched_proc_proto_uproc_proto_msgTypes,
	}.Build()
	File_sched_msched_proc_proto_uproc_proto = out.File
	file_sched_msched_proc_proto_uproc_proto_rawDesc = nil
	file_sched_msched_proc_proto_uproc_proto_goTypes = nil
	file_sched_msched_proc_proto_uproc_proto_depIdxs = nil
}
