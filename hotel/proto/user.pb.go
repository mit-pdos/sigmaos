// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v3.12.4
// source: hotel/proto/user.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proto "sigmaos/tracing/proto"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type UserRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name              string                   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Password          string                   `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	SpanContextConfig *proto.SpanContextConfig `protobuf:"bytes,3,opt,name=spanContextConfig,proto3" json:"spanContextConfig,omitempty"`
}

func (x *UserRequest) Reset() {
	*x = UserRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_user_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserRequest) ProtoMessage() {}

func (x *UserRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_user_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserRequest.ProtoReflect.Descriptor instead.
func (*UserRequest) Descriptor() ([]byte, []int) {
	return file_hotel_proto_user_proto_rawDescGZIP(), []int{0}
}

func (x *UserRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *UserRequest) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *UserRequest) GetSpanContextConfig() *proto.SpanContextConfig {
	if x != nil {
		return x.SpanContextConfig
	}
	return nil
}

type UserResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OK string `protobuf:"bytes,1,opt,name=oK,proto3" json:"oK,omitempty"`
}

func (x *UserResult) Reset() {
	*x = UserResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_user_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserResult) ProtoMessage() {}

func (x *UserResult) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_user_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserResult.ProtoReflect.Descriptor instead.
func (*UserResult) Descriptor() ([]byte, []int) {
	return file_hotel_proto_user_proto_rawDescGZIP(), []int{1}
}

func (x *UserResult) GetOK() string {
	if x != nil {
		return x.OK
	}
	return ""
}

var File_hotel_proto_user_proto protoreflect.FileDescriptor

var file_hotel_proto_user_proto_rawDesc = []byte{
	0x0a, 0x16, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x75, 0x73,
	0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e,
	0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x7f, 0x0a, 0x0b, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x73, 0x73,
	0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x61, 0x73, 0x73,
	0x77, 0x6f, 0x72, 0x64, 0x12, 0x40, 0x0a, 0x11, 0x73, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74,
	0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x12, 0x2e, 0x53, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x52, 0x11, 0x73, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x22, 0x1c, 0x0a, 0x0a, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x4b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x6f, 0x4b, 0x32, 0x64, 0x0a, 0x04, 0x55, 0x73, 0x65, 0x72, 0x12, 0x2c, 0x0a, 0x0f,
	0x4d, 0x61, 0x6b, 0x65, 0x52, 0x65, 0x73, 0x65, 0x72, 0x76, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x0c, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0b, 0x2e,
	0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x2e, 0x0a, 0x11, 0x43, 0x68,
	0x65, 0x63, 0x6b, 0x41, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x12,
	0x0c, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0b, 0x2e,
	0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x42, 0x15, 0x5a, 0x13, 0x73, 0x69,
	0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_hotel_proto_user_proto_rawDescOnce sync.Once
	file_hotel_proto_user_proto_rawDescData = file_hotel_proto_user_proto_rawDesc
)

func file_hotel_proto_user_proto_rawDescGZIP() []byte {
	file_hotel_proto_user_proto_rawDescOnce.Do(func() {
		file_hotel_proto_user_proto_rawDescData = protoimpl.X.CompressGZIP(file_hotel_proto_user_proto_rawDescData)
	})
	return file_hotel_proto_user_proto_rawDescData
}

var file_hotel_proto_user_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_hotel_proto_user_proto_goTypes = []interface{}{
	(*UserRequest)(nil),             // 0: UserRequest
	(*UserResult)(nil),              // 1: UserResult
	(*proto.SpanContextConfig)(nil), // 2: SpanContextConfig
}
var file_hotel_proto_user_proto_depIdxs = []int32{
	2, // 0: UserRequest.spanContextConfig:type_name -> SpanContextConfig
	0, // 1: User.MakeReservation:input_type -> UserRequest
	0, // 2: User.CheckAvailability:input_type -> UserRequest
	1, // 3: User.MakeReservation:output_type -> UserResult
	1, // 4: User.CheckAvailability:output_type -> UserResult
	3, // [3:5] is the sub-list for method output_type
	1, // [1:3] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_hotel_proto_user_proto_init() }
func file_hotel_proto_user_proto_init() {
	if File_hotel_proto_user_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_hotel_proto_user_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserRequest); i {
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
		file_hotel_proto_user_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserResult); i {
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
			RawDescriptor: file_hotel_proto_user_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_hotel_proto_user_proto_goTypes,
		DependencyIndexes: file_hotel_proto_user_proto_depIdxs,
		MessageInfos:      file_hotel_proto_user_proto_msgTypes,
	}.Build()
	File_hotel_proto_user_proto = out.File
	file_hotel_proto_user_proto_rawDesc = nil
	file_hotel_proto_user_proto_goTypes = nil
	file_hotel_proto_user_proto_depIdxs = nil
}
