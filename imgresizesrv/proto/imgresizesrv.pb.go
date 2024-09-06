// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: imgresizesrv/proto/imgresizesrv.proto

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

type ImgResizeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TaskName  string `protobuf:"bytes,1,opt,name=taskName,proto3" json:"taskName,omitempty"`
	InputPath string `protobuf:"bytes,2,opt,name=inputPath,proto3" json:"inputPath,omitempty"`
}

func (x *ImgResizeRequest) Reset() {
	*x = ImgResizeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ImgResizeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ImgResizeRequest) ProtoMessage() {}

func (x *ImgResizeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ImgResizeRequest.ProtoReflect.Descriptor instead.
func (*ImgResizeRequest) Descriptor() ([]byte, []int) {
	return file_imgresizesrv_proto_imgresizesrv_proto_rawDescGZIP(), []int{0}
}

func (x *ImgResizeRequest) GetTaskName() string {
	if x != nil {
		return x.TaskName
	}
	return ""
}

func (x *ImgResizeRequest) GetInputPath() string {
	if x != nil {
		return x.InputPath
	}
	return ""
}

type ImgResizeResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OK bool `protobuf:"varint,1,opt,name=oK,proto3" json:"oK,omitempty"`
}

func (x *ImgResizeResult) Reset() {
	*x = ImgResizeResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ImgResizeResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ImgResizeResult) ProtoMessage() {}

func (x *ImgResizeResult) ProtoReflect() protoreflect.Message {
	mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ImgResizeResult.ProtoReflect.Descriptor instead.
func (*ImgResizeResult) Descriptor() ([]byte, []int) {
	return file_imgresizesrv_proto_imgresizesrv_proto_rawDescGZIP(), []int{1}
}

func (x *ImgResizeResult) GetOK() bool {
	if x != nil {
		return x.OK
	}
	return false
}

type StatusRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StatusRequest) Reset() {
	*x = StatusRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatusRequest) ProtoMessage() {}

func (x *StatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatusRequest.ProtoReflect.Descriptor instead.
func (*StatusRequest) Descriptor() ([]byte, []int) {
	return file_imgresizesrv_proto_imgresizesrv_proto_rawDescGZIP(), []int{2}
}

type StatusResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NDone int64 `protobuf:"varint,1,opt,name=nDone,proto3" json:"nDone,omitempty"`
}

func (x *StatusResult) Reset() {
	*x = StatusResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StatusResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatusResult) ProtoMessage() {}

func (x *StatusResult) ProtoReflect() protoreflect.Message {
	mi := &file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatusResult.ProtoReflect.Descriptor instead.
func (*StatusResult) Descriptor() ([]byte, []int) {
	return file_imgresizesrv_proto_imgresizesrv_proto_rawDescGZIP(), []int{3}
}

func (x *StatusResult) GetNDone() int64 {
	if x != nil {
		return x.NDone
	}
	return 0
}

var File_imgresizesrv_proto_imgresizesrv_proto protoreflect.FileDescriptor

var file_imgresizesrv_proto_imgresizesrv_proto_rawDesc = []byte{
	0x0a, 0x25, 0x69, 0x6d, 0x67, 0x72, 0x65, 0x73, 0x69, 0x7a, 0x65, 0x73, 0x72, 0x76, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x69, 0x6d, 0x67, 0x72, 0x65, 0x73, 0x69, 0x7a, 0x65, 0x73, 0x72,
	0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4c, 0x0a, 0x10, 0x49, 0x6d, 0x67, 0x52, 0x65,
	0x73, 0x69, 0x7a, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x74,
	0x61, 0x73, 0x6b, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74,
	0x61, 0x73, 0x6b, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x69, 0x6e, 0x70, 0x75, 0x74,
	0x50, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x69, 0x6e, 0x70, 0x75,
	0x74, 0x50, 0x61, 0x74, 0x68, 0x22, 0x21, 0x0a, 0x0f, 0x49, 0x6d, 0x67, 0x52, 0x65, 0x73, 0x69,
	0x7a, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x4b, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x02, 0x6f, 0x4b, 0x22, 0x0f, 0x0a, 0x0d, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x24, 0x0a, 0x0c, 0x53, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x6e, 0x44, 0x6f,
	0x6e, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x6e, 0x44, 0x6f, 0x6e, 0x65, 0x42,
	0x1c, 0x5a, 0x1a, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x69, 0x6d, 0x67, 0x72, 0x65,
	0x73, 0x69, 0x7a, 0x65, 0x73, 0x72, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_imgresizesrv_proto_imgresizesrv_proto_rawDescOnce sync.Once
	file_imgresizesrv_proto_imgresizesrv_proto_rawDescData = file_imgresizesrv_proto_imgresizesrv_proto_rawDesc
)

func file_imgresizesrv_proto_imgresizesrv_proto_rawDescGZIP() []byte {
	file_imgresizesrv_proto_imgresizesrv_proto_rawDescOnce.Do(func() {
		file_imgresizesrv_proto_imgresizesrv_proto_rawDescData = protoimpl.X.CompressGZIP(file_imgresizesrv_proto_imgresizesrv_proto_rawDescData)
	})
	return file_imgresizesrv_proto_imgresizesrv_proto_rawDescData
}

var file_imgresizesrv_proto_imgresizesrv_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_imgresizesrv_proto_imgresizesrv_proto_goTypes = []interface{}{
	(*ImgResizeRequest)(nil), // 0: ImgResizeRequest
	(*ImgResizeResult)(nil),  // 1: ImgResizeResult
	(*StatusRequest)(nil),    // 2: StatusRequest
	(*StatusResult)(nil),     // 3: StatusResult
}
var file_imgresizesrv_proto_imgresizesrv_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_imgresizesrv_proto_imgresizesrv_proto_init() }
func file_imgresizesrv_proto_imgresizesrv_proto_init() {
	if File_imgresizesrv_proto_imgresizesrv_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ImgResizeRequest); i {
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
		file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ImgResizeResult); i {
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
		file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StatusRequest); i {
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
		file_imgresizesrv_proto_imgresizesrv_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StatusResult); i {
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
			RawDescriptor: file_imgresizesrv_proto_imgresizesrv_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_imgresizesrv_proto_imgresizesrv_proto_goTypes,
		DependencyIndexes: file_imgresizesrv_proto_imgresizesrv_proto_depIdxs,
		MessageInfos:      file_imgresizesrv_proto_imgresizesrv_proto_msgTypes,
	}.Build()
	File_imgresizesrv_proto_imgresizesrv_proto = out.File
	file_imgresizesrv_proto_imgresizesrv_proto_rawDesc = nil
	file_imgresizesrv_proto_imgresizesrv_proto_goTypes = nil
	file_imgresizesrv_proto_imgresizesrv_proto_depIdxs = nil
}
