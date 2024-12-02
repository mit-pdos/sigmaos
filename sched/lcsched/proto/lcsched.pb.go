// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: sched/lcsched/proto/lcsched.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proto "sigmaos/sched/besched/proto"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RegisterMSchedRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KernelID string `protobuf:"bytes,1,opt,name=kernelID,proto3" json:"kernelID,omitempty"`
	McpuInt  uint32 `protobuf:"varint,2,opt,name=mcpuInt,proto3" json:"mcpuInt,omitempty"`
	MemInt   uint32 `protobuf:"varint,3,opt,name=memInt,proto3" json:"memInt,omitempty"`
}

func (x *RegisterMSchedRequest) Reset() {
	*x = RegisterMSchedRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_lcsched_proto_lcsched_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterMSchedRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterMSchedRequest) ProtoMessage() {}

func (x *RegisterMSchedRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sched_lcsched_proto_lcsched_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterMSchedRequest.ProtoReflect.Descriptor instead.
func (*RegisterMSchedRequest) Descriptor() ([]byte, []int) {
	return file_sched_lcsched_proto_lcsched_proto_rawDescGZIP(), []int{0}
}

func (x *RegisterMSchedRequest) GetKernelID() string {
	if x != nil {
		return x.KernelID
	}
	return ""
}

func (x *RegisterMSchedRequest) GetMcpuInt() uint32 {
	if x != nil {
		return x.McpuInt
	}
	return 0
}

func (x *RegisterMSchedRequest) GetMemInt() uint32 {
	if x != nil {
		return x.MemInt
	}
	return 0
}

type RegisterMSchedResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RegisterMSchedResponse) Reset() {
	*x = RegisterMSchedResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sched_lcsched_proto_lcsched_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterMSchedResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterMSchedResponse) ProtoMessage() {}

func (x *RegisterMSchedResponse) ProtoReflect() protoreflect.Message {
	mi := &file_sched_lcsched_proto_lcsched_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterMSchedResponse.ProtoReflect.Descriptor instead.
func (*RegisterMSchedResponse) Descriptor() ([]byte, []int) {
	return file_sched_lcsched_proto_lcsched_proto_rawDescGZIP(), []int{1}
}

var File_sched_lcsched_proto_lcsched_proto protoreflect.FileDescriptor

var file_sched_lcsched_proto_lcsched_proto_rawDesc = []byte{
	0x0a, 0x21, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x6c, 0x63, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6c, 0x63, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x62, 0x65, 0x73, 0x63, 0x68,
	0x65, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x65, 0x73, 0x63, 0x68, 0x65, 0x64,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x65, 0x0a, 0x15, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74,
	0x65, 0x72, 0x4d, 0x53, 0x63, 0x68, 0x65, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x1a, 0x0a, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x12, 0x18, 0x0a, 0x07, 0x6d,
	0x63, 0x70, 0x75, 0x49, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x07, 0x6d, 0x63,
	0x70, 0x75, 0x49, 0x6e, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x6d, 0x49, 0x6e, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x06, 0x6d, 0x65, 0x6d, 0x49, 0x6e, 0x74, 0x22, 0x18, 0x0a,
	0x16, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x4d, 0x53, 0x63, 0x68, 0x65, 0x64, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x7a, 0x0a, 0x07, 0x4c, 0x43, 0x53, 0x63, 0x68,
	0x65, 0x64, 0x12, 0x2c, 0x0a, 0x07, 0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x12, 0x0f, 0x2e,
	0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10,
	0x2e, 0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x41, 0x0a, 0x0e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x4d, 0x53, 0x63, 0x68,
	0x65, 0x64, 0x12, 0x16, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x4d, 0x53, 0x63,
	0x68, 0x65, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x52, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x65, 0x72, 0x4d, 0x53, 0x63, 0x68, 0x65, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x42, 0x1d, 0x5a, 0x1b, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x73,
	0x63, 0x68, 0x65, 0x64, 0x2f, 0x6c, 0x63, 0x73, 0x63, 0x68, 0x65, 0x64, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sched_lcsched_proto_lcsched_proto_rawDescOnce sync.Once
	file_sched_lcsched_proto_lcsched_proto_rawDescData = file_sched_lcsched_proto_lcsched_proto_rawDesc
)

func file_sched_lcsched_proto_lcsched_proto_rawDescGZIP() []byte {
	file_sched_lcsched_proto_lcsched_proto_rawDescOnce.Do(func() {
		file_sched_lcsched_proto_lcsched_proto_rawDescData = protoimpl.X.CompressGZIP(file_sched_lcsched_proto_lcsched_proto_rawDescData)
	})
	return file_sched_lcsched_proto_lcsched_proto_rawDescData
}

var file_sched_lcsched_proto_lcsched_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_sched_lcsched_proto_lcsched_proto_goTypes = []interface{}{
	(*RegisterMSchedRequest)(nil),  // 0: RegisterMSchedRequest
	(*RegisterMSchedResponse)(nil), // 1: RegisterMSchedResponse
	(*proto.EnqueueRequest)(nil),   // 2: EnqueueRequest
	(*proto.EnqueueResponse)(nil),  // 3: EnqueueResponse
}
var file_sched_lcsched_proto_lcsched_proto_depIdxs = []int32{
	2, // 0: LCSched.Enqueue:input_type -> EnqueueRequest
	0, // 1: LCSched.RegisterMSched:input_type -> RegisterMSchedRequest
	3, // 2: LCSched.Enqueue:output_type -> EnqueueResponse
	1, // 3: LCSched.RegisterMSched:output_type -> RegisterMSchedResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_sched_lcsched_proto_lcsched_proto_init() }
func file_sched_lcsched_proto_lcsched_proto_init() {
	if File_sched_lcsched_proto_lcsched_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sched_lcsched_proto_lcsched_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RegisterMSchedRequest); i {
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
		file_sched_lcsched_proto_lcsched_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RegisterMSchedResponse); i {
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
			RawDescriptor: file_sched_lcsched_proto_lcsched_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_sched_lcsched_proto_lcsched_proto_goTypes,
		DependencyIndexes: file_sched_lcsched_proto_lcsched_proto_depIdxs,
		MessageInfos:      file_sched_lcsched_proto_lcsched_proto_msgTypes,
	}.Build()
	File_sched_lcsched_proto_lcsched_proto = out.File
	file_sched_lcsched_proto_lcsched_proto_rawDesc = nil
	file_sched_lcsched_proto_lcsched_proto_goTypes = nil
	file_sched_lcsched_proto_lcsched_proto_depIdxs = nil
}