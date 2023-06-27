// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.12
// source: k8sutil/proto/k8sutil.proto

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

type CPUUtilRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	QoSClass string `protobuf:"bytes,1,opt,name=qoSClass,proto3" json:"qoSClass,omitempty"`
}

func (x *CPUUtilRequest) Reset() {
	*x = CPUUtilRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_k8sutil_proto_k8sutil_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CPUUtilRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CPUUtilRequest) ProtoMessage() {}

func (x *CPUUtilRequest) ProtoReflect() protoreflect.Message {
	mi := &file_k8sutil_proto_k8sutil_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CPUUtilRequest.ProtoReflect.Descriptor instead.
func (*CPUUtilRequest) Descriptor() ([]byte, []int) {
	return file_k8sutil_proto_k8sutil_proto_rawDescGZIP(), []int{0}
}

func (x *CPUUtilRequest) GetQoSClass() string {
	if x != nil {
		return x.QoSClass
	}
	return ""
}

type CPUUtilResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Util uint64 `protobuf:"varint,1,opt,name=util,proto3" json:"util,omitempty"`
}

func (x *CPUUtilResult) Reset() {
	*x = CPUUtilResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_k8sutil_proto_k8sutil_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CPUUtilResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CPUUtilResult) ProtoMessage() {}

func (x *CPUUtilResult) ProtoReflect() protoreflect.Message {
	mi := &file_k8sutil_proto_k8sutil_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CPUUtilResult.ProtoReflect.Descriptor instead.
func (*CPUUtilResult) Descriptor() ([]byte, []int) {
	return file_k8sutil_proto_k8sutil_proto_rawDescGZIP(), []int{1}
}

func (x *CPUUtilResult) GetUtil() uint64 {
	if x != nil {
		return x.Util
	}
	return 0
}

var File_k8sutil_proto_k8sutil_proto protoreflect.FileDescriptor

var file_k8sutil_proto_k8sutil_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x6b, 0x38, 0x73, 0x75, 0x74, 0x69, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x6b, 0x38, 0x73, 0x75, 0x74, 0x69, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2c, 0x0a,
	0x0e, 0x43, 0x50, 0x55, 0x55, 0x74, 0x69, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x1a, 0x0a, 0x08, 0x71, 0x6f, 0x53, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x71, 0x6f, 0x53, 0x43, 0x6c, 0x61, 0x73, 0x73, 0x22, 0x23, 0x0a, 0x0d, 0x43,
	0x50, 0x55, 0x55, 0x74, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x12, 0x0a, 0x04,
	0x75, 0x74, 0x69, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x75, 0x74, 0x69, 0x6c,
	0x42, 0x17, 0x5a, 0x15, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x6b, 0x38, 0x73, 0x75,
	0x74, 0x69, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_k8sutil_proto_k8sutil_proto_rawDescOnce sync.Once
	file_k8sutil_proto_k8sutil_proto_rawDescData = file_k8sutil_proto_k8sutil_proto_rawDesc
)

func file_k8sutil_proto_k8sutil_proto_rawDescGZIP() []byte {
	file_k8sutil_proto_k8sutil_proto_rawDescOnce.Do(func() {
		file_k8sutil_proto_k8sutil_proto_rawDescData = protoimpl.X.CompressGZIP(file_k8sutil_proto_k8sutil_proto_rawDescData)
	})
	return file_k8sutil_proto_k8sutil_proto_rawDescData
}

var file_k8sutil_proto_k8sutil_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_k8sutil_proto_k8sutil_proto_goTypes = []interface{}{
	(*CPUUtilRequest)(nil), // 0: CPUUtilRequest
	(*CPUUtilResult)(nil),  // 1: CPUUtilResult
}
var file_k8sutil_proto_k8sutil_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_k8sutil_proto_k8sutil_proto_init() }
func file_k8sutil_proto_k8sutil_proto_init() {
	if File_k8sutil_proto_k8sutil_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_k8sutil_proto_k8sutil_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CPUUtilRequest); i {
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
		file_k8sutil_proto_k8sutil_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CPUUtilResult); i {
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
			RawDescriptor: file_k8sutil_proto_k8sutil_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_k8sutil_proto_k8sutil_proto_goTypes,
		DependencyIndexes: file_k8sutil_proto_k8sutil_proto_depIdxs,
		MessageInfos:      file_k8sutil_proto_k8sutil_proto_msgTypes,
	}.Build()
	File_k8sutil_proto_k8sutil_proto = out.File
	file_k8sutil_proto_k8sutil_proto_rawDesc = nil
	file_k8sutil_proto_k8sutil_proto_goTypes = nil
	file_k8sutil_proto_k8sutil_proto_depIdxs = nil
}
