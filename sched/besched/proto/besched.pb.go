// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: sched/besched/proto/besched.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proc "sigmaos/proc"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EnqueueReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProcProto *proc.ProcProto `protobuf:"bytes,1,opt,name=procProto,proto3" json:"procProto,omitempty"`
}

func (x *EnqueueReq) Reset() {
	*x = EnqueueReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnqueueReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnqueueReq) ProtoMessage() {}

func (x *EnqueueReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnqueueReq.ProtoReflect.Descriptor instead.
func (*EnqueueReq) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{0}
}

func (x *EnqueueReq) GetProcProto() *proc.ProcProto {
	if x != nil {
		return x.ProcProto
	}
	return nil
}

type EnqueueRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MSchedID  string          `protobuf:"bytes,1,opt,name=mSchedID,proto3" json:"mSchedID,omitempty"`
	ProcSeqno *proc.ProcSeqno `protobuf:"bytes,2,opt,name=procSeqno,proto3" json:"procSeqno,omitempty"`
}

func (x *EnqueueRep) Reset() {
	*x = EnqueueRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnqueueRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnqueueRep) ProtoMessage() {}

func (x *EnqueueRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnqueueRep.ProtoReflect.Descriptor instead.
func (*EnqueueRep) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{1}
}

func (x *EnqueueRep) GetMSchedID() string {
	if x != nil {
		return x.MSchedID
	}
	return ""
}

func (x *EnqueueRep) GetProcSeqno() *proc.ProcSeqno {
	if x != nil {
		return x.ProcSeqno
	}
	return nil
}

type GetProcReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KernelID  string          `protobuf:"bytes,1,opt,name=kernelID,proto3" json:"kernelID,omitempty"`
	Mem       uint32          `protobuf:"varint,2,opt,name=mem,proto3" json:"mem,omitempty"`
	ProcSeqno *proc.ProcSeqno `protobuf:"bytes,3,opt,name=procSeqno,proto3" json:"procSeqno,omitempty"`
}

func (x *GetProcReq) Reset() {
	*x = GetProcReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProcReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProcReq) ProtoMessage() {}

func (x *GetProcReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProcReq.ProtoReflect.Descriptor instead.
func (*GetProcReq) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{2}
}

func (x *GetProcReq) GetKernelID() string {
	if x != nil {
		return x.KernelID
	}
	return ""
}

func (x *GetProcReq) GetMem() uint32 {
	if x != nil {
		return x.Mem
	}
	return 0
}

func (x *GetProcReq) GetProcSeqno() *proc.ProcSeqno {
	if x != nil {
		return x.ProcSeqno
	}
	return nil
}

type GetProcRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OK        bool            `protobuf:"varint,1,opt,name=oK,proto3" json:"oK,omitempty"`
	QLen      uint32          `protobuf:"varint,2,opt,name=qLen,proto3" json:"qLen,omitempty"`
	ProcProto *proc.ProcProto `protobuf:"bytes,3,opt,name=procProto,proto3" json:"procProto,omitempty"`
}

func (x *GetProcRep) Reset() {
	*x = GetProcRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProcRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProcRep) ProtoMessage() {}

func (x *GetProcRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProcRep.ProtoReflect.Descriptor instead.
func (*GetProcRep) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{3}
}

func (x *GetProcRep) GetOK() bool {
	if x != nil {
		return x.OK
	}
	return false
}

func (x *GetProcRep) GetQLen() uint32 {
	if x != nil {
		return x.QLen
	}
	return 0
}

func (x *GetProcRep) GetProcProto() *proc.ProcProto {
	if x != nil {
		return x.ProcProto
	}
	return nil
}

type GetStatsReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetStatsReq) Reset() {
	*x = GetStatsReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStatsReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatsReq) ProtoMessage() {}

func (x *GetStatsReq) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatsReq.ProtoReflect.Descriptor instead.
func (*GetStatsReq) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{4}
}

type GetStatsRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Nqueued map[string]int64 `protobuf:"bytes,1,rep,name=nqueued,proto3" json:"nqueued,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
}

func (x *GetStatsRep) Reset() {
	*x = GetStatsRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_besched_proto_besched_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetStatsRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetStatsRep) ProtoMessage() {}

func (x *GetStatsRep) ProtoReflect() protoreflect.Message {
	mi := &file_sched_besched_proto_besched_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetStatsRep.ProtoReflect.Descriptor instead.
func (*GetStatsRep) Descriptor() ([]byte, []int) {
	return file_sched_besched_proto_besched_proto_rawDescGZIP(), []int{5}
}

func (x *GetStatsRep) GetNqueued() map[string]int64 {
	if x != nil {
		return x.Nqueued
	}
	return nil
}

var File_sched_besched_proto_besched_proto protoreflect.FileDescriptor

var file_sched_besched_proto_besched_proto_rawDesc = []byte{
	0x0a, 0x21, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x62, 0x65, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x65, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x0f, 0x70, 0x72, 0x6f, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x63, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x36, 0x0a, 0x0a, 0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52,
	0x65, 0x71, 0x12, 0x28, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x52, 0x0a, 0x0a,
	0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x6d, 0x53,
	0x63, 0x68, 0x65, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6d, 0x53,
	0x63, 0x68, 0x65, 0x64, 0x49, 0x44, 0x12, 0x28, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x53, 0x65,
	0x71, 0x6e, 0x6f, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63,
	0x53, 0x65, 0x71, 0x6e, 0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x53, 0x65, 0x71, 0x6e, 0x6f,
	0x22, 0x64, 0x0a, 0x0a, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f, 0x63, 0x52, 0x65, 0x71, 0x12, 0x1a,
	0x0a, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x65,
	0x6d, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x6d, 0x65, 0x6d, 0x12, 0x28, 0x0a, 0x09,
	0x70, 0x72, 0x6f, 0x63, 0x53, 0x65, 0x71, 0x6e, 0x6f, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x53, 0x65, 0x71, 0x6e, 0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f,
	0x63, 0x53, 0x65, 0x71, 0x6e, 0x6f, 0x22, 0x5a, 0x0a, 0x0a, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f,
	0x63, 0x52, 0x65, 0x70, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x4b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x02, 0x6f, 0x4b, 0x12, 0x12, 0x0a, 0x04, 0x71, 0x4c, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x04, 0x71, 0x4c, 0x65, 0x6e, 0x12, 0x28, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x50, 0x72,
	0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0x0d, 0x0a, 0x0b, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x73, 0x52, 0x65,
	0x71, 0x22, 0x7e, 0x0a, 0x0b, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x73, 0x52, 0x65, 0x70,
	0x12, 0x33, 0x0a, 0x07, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x19, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x74, 0x61, 0x74, 0x73, 0x52, 0x65, 0x70, 0x2e,
	0x4e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x64, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x6e, 0x71,
	0x75, 0x65, 0x75, 0x65, 0x64, 0x1a, 0x3a, 0x0a, 0x0c, 0x4e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x64,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38,
	0x01, 0x42, 0x1d, 0x5a, 0x1b, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x73, 0x63, 0x68,
	0x65, 0x64, 0x2f, 0x62, 0x65, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sched_besched_proto_besched_proto_rawDescOnce sync.Once
	file_sched_besched_proto_besched_proto_rawDescData = file_sched_besched_proto_besched_proto_rawDesc
)

func file_sched_besched_proto_besched_proto_rawDescGZIP() []byte {
	file_sched_besched_proto_besched_proto_rawDescOnce.Do(func() {
		file_sched_besched_proto_besched_proto_rawDescData = protoimpl.X.CompressGZIP(file_sched_besched_proto_besched_proto_rawDescData)
	})
	return file_sched_besched_proto_besched_proto_rawDescData
}

var file_sched_besched_proto_besched_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_sched_besched_proto_besched_proto_goTypes = []interface{}{
	(*EnqueueReq)(nil),     // 0: EnqueueReq
	(*EnqueueRep)(nil),     // 1: EnqueueRep
	(*GetProcReq)(nil),     // 2: GetProcReq
	(*GetProcRep)(nil),     // 3: GetProcRep
	(*GetStatsReq)(nil),    // 4: GetStatsReq
	(*GetStatsRep)(nil),    // 5: GetStatsRep
	nil,                    // 6: GetStatsRep.NqueuedEntry
	(*proc.ProcProto)(nil), // 7: ProcProto
	(*proc.ProcSeqno)(nil), // 8: ProcSeqno
}
var file_sched_besched_proto_besched_proto_depIdxs = []int32{
	7, // 0: EnqueueReq.procProto:type_name -> ProcProto
	8, // 1: EnqueueRep.procSeqno:type_name -> ProcSeqno
	8, // 2: GetProcReq.procSeqno:type_name -> ProcSeqno
	7, // 3: GetProcRep.procProto:type_name -> ProcProto
	6, // 4: GetStatsRep.nqueued:type_name -> GetStatsRep.NqueuedEntry
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_sched_besched_proto_besched_proto_init() }
func file_sched_besched_proto_besched_proto_init() {
	if File_sched_besched_proto_besched_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sched_besched_proto_besched_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnqueueReq); i {
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
		file_sched_besched_proto_besched_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnqueueRep); i {
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
		file_sched_besched_proto_besched_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProcReq); i {
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
		file_sched_besched_proto_besched_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProcRep); i {
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
		file_sched_besched_proto_besched_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStatsReq); i {
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
		file_sched_besched_proto_besched_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetStatsRep); i {
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
			RawDescriptor: file_sched_besched_proto_besched_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sched_besched_proto_besched_proto_goTypes,
		DependencyIndexes: file_sched_besched_proto_besched_proto_depIdxs,
		MessageInfos:      file_sched_besched_proto_besched_proto_msgTypes,
	}.Build()
	File_sched_besched_proto_besched_proto = out.File
	file_sched_besched_proto_besched_proto_rawDesc = nil
	file_sched_besched_proto_besched_proto_goTypes = nil
	file_sched_besched_proto_besched_proto_depIdxs = nil
}
