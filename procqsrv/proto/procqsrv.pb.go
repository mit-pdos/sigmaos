// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: procqsrv/proto/procqsrv.proto

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

type EnqueueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProcProto *proc.ProcProto `protobuf:"bytes,1,opt,name=procProto,proto3" json:"procProto,omitempty"`
}

func (x *EnqueueRequest) Reset() {
	*x = EnqueueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnqueueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnqueueRequest) ProtoMessage() {}

func (x *EnqueueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnqueueRequest.ProtoReflect.Descriptor instead.
func (*EnqueueRequest) Descriptor() ([]byte, []int) {
	return file_procqsrv_proto_procqsrv_proto_rawDescGZIP(), []int{0}
}

func (x *EnqueueRequest) GetProcProto() *proc.ProcProto {
	if x != nil {
		return x.ProcProto
	}
	return nil
}

type EnqueueResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KernelID string `protobuf:"bytes,1,opt,name=kernelID,proto3" json:"kernelID,omitempty"`
}

func (x *EnqueueResponse) Reset() {
	*x = EnqueueResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnqueueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnqueueResponse) ProtoMessage() {}

func (x *EnqueueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnqueueResponse.ProtoReflect.Descriptor instead.
func (*EnqueueResponse) Descriptor() ([]byte, []int) {
	return file_procqsrv_proto_procqsrv_proto_rawDescGZIP(), []int{1}
}

func (x *EnqueueResponse) GetKernelID() string {
	if x != nil {
		return x.KernelID
	}
	return ""
}

type GetProcRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	KernelID string `protobuf:"bytes,1,opt,name=kernelID,proto3" json:"kernelID,omitempty"`
}

func (x *GetProcRequest) Reset() {
	*x = GetProcRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProcRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProcRequest) ProtoMessage() {}

func (x *GetProcRequest) ProtoReflect() protoreflect.Message {
	mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProcRequest.ProtoReflect.Descriptor instead.
func (*GetProcRequest) Descriptor() ([]byte, []int) {
	return file_procqsrv_proto_procqsrv_proto_rawDescGZIP(), []int{2}
}

func (x *GetProcRequest) GetKernelID() string {
	if x != nil {
		return x.KernelID
	}
	return ""
}

type GetProcResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ProcProto *proc.ProcProto `protobuf:"bytes,1,opt,name=procProto,proto3" json:"procProto,omitempty"`
}

func (x *GetProcResponse) Reset() {
	*x = GetProcResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProcResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProcResponse) ProtoMessage() {}

func (x *GetProcResponse) ProtoReflect() protoreflect.Message {
	mi := &file_procqsrv_proto_procqsrv_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProcResponse.ProtoReflect.Descriptor instead.
func (*GetProcResponse) Descriptor() ([]byte, []int) {
	return file_procqsrv_proto_procqsrv_proto_rawDescGZIP(), []int{3}
}

func (x *GetProcResponse) GetProcProto() *proc.ProcProto {
	if x != nil {
		return x.ProcProto
	}
	return nil
}

var File_procqsrv_proto_procqsrv_proto protoreflect.FileDescriptor

var file_procqsrv_proto_procqsrv_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x70, 0x72, 0x6f, 0x63, 0x71, 0x73, 0x72, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x70, 0x72, 0x6f, 0x63, 0x71, 0x73, 0x72, 0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x0f, 0x70, 0x72, 0x6f, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x3a, 0x0a, 0x0e, 0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x28, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2d, 0x0a, 0x0f,
	0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x1a, 0x0a, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x22, 0x2c, 0x0a, 0x0e, 0x47,
	0x65, 0x74, 0x50, 0x72, 0x6f, 0x63, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a,
	0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x49, 0x44, 0x22, 0x3b, 0x0a, 0x0f, 0x47, 0x65, 0x74,
	0x50, 0x72, 0x6f, 0x63, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x28, 0x0a, 0x09,
	0x70, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x0a, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x52, 0x09, 0x70, 0x72, 0x6f,
	0x63, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x32, 0x63, 0x0a, 0x05, 0x50, 0x72, 0x6f, 0x63, 0x51, 0x12,
	0x2c, 0x0a, 0x07, 0x45, 0x6e, 0x71, 0x75, 0x65, 0x75, 0x65, 0x12, 0x0f, 0x2e, 0x45, 0x6e, 0x71,
	0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x45, 0x6e,
	0x71, 0x75, 0x65, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2c, 0x0a,
	0x07, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f, 0x63, 0x12, 0x0f, 0x2e, 0x47, 0x65, 0x74, 0x50, 0x72,
	0x6f, 0x63, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x47, 0x65, 0x74, 0x50,
	0x72, 0x6f, 0x63, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x18, 0x5a, 0x16, 0x73,
	0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x63, 0x71, 0x73, 0x72, 0x76, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_procqsrv_proto_procqsrv_proto_rawDescOnce sync.Once
	file_procqsrv_proto_procqsrv_proto_rawDescData = file_procqsrv_proto_procqsrv_proto_rawDesc
)

func file_procqsrv_proto_procqsrv_proto_rawDescGZIP() []byte {
	file_procqsrv_proto_procqsrv_proto_rawDescOnce.Do(func() {
		file_procqsrv_proto_procqsrv_proto_rawDescData = protoimpl.X.CompressGZIP(file_procqsrv_proto_procqsrv_proto_rawDescData)
	})
	return file_procqsrv_proto_procqsrv_proto_rawDescData
}

var file_procqsrv_proto_procqsrv_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_procqsrv_proto_procqsrv_proto_goTypes = []interface{}{
	(*EnqueueRequest)(nil),  // 0: EnqueueRequest
	(*EnqueueResponse)(nil), // 1: EnqueueResponse
	(*GetProcRequest)(nil),  // 2: GetProcRequest
	(*GetProcResponse)(nil), // 3: GetProcResponse
	(*proc.ProcProto)(nil),  // 4: ProcProto
}
var file_procqsrv_proto_procqsrv_proto_depIdxs = []int32{
	4, // 0: EnqueueRequest.procProto:type_name -> ProcProto
	4, // 1: GetProcResponse.procProto:type_name -> ProcProto
	0, // 2: ProcQ.Enqueue:input_type -> EnqueueRequest
	2, // 3: ProcQ.GetProc:input_type -> GetProcRequest
	1, // 4: ProcQ.Enqueue:output_type -> EnqueueResponse
	3, // 5: ProcQ.GetProc:output_type -> GetProcResponse
	4, // [4:6] is the sub-list for method output_type
	2, // [2:4] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_procqsrv_proto_procqsrv_proto_init() }
func file_procqsrv_proto_procqsrv_proto_init() {
	if File_procqsrv_proto_procqsrv_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_procqsrv_proto_procqsrv_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnqueueRequest); i {
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
		file_procqsrv_proto_procqsrv_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnqueueResponse); i {
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
		file_procqsrv_proto_procqsrv_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProcRequest); i {
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
		file_procqsrv_proto_procqsrv_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProcResponse); i {
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
			RawDescriptor: file_procqsrv_proto_procqsrv_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_procqsrv_proto_procqsrv_proto_goTypes,
		DependencyIndexes: file_procqsrv_proto_procqsrv_proto_depIdxs,
		MessageInfos:      file_procqsrv_proto_procqsrv_proto_msgTypes,
	}.Build()
	File_procqsrv_proto_procqsrv_proto = out.File
	file_procqsrv_proto_procqsrv_proto_rawDesc = nil
	file_procqsrv_proto_procqsrv_proto_goTypes = nil
	file_procqsrv_proto_procqsrv_proto_depIdxs = nil
}