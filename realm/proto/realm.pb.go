// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: realm/proto/realm.proto

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

type MakeReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Realm   string `protobuf:"bytes,1,opt,name=realm,proto3" json:"realm,omitempty"`
	Network string `protobuf:"bytes,2,opt,name=network,proto3" json:"network,omitempty"`
	NumS3   int64  `protobuf:"varint,3,opt,name=numS3,proto3" json:"numS3,omitempty"`
	NumUX   int64  `protobuf:"varint,4,opt,name=numUX,proto3" json:"numUX,omitempty"`
}

func (x *MakeReq) Reset() {
	*x = MakeReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_realm_proto_realm_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MakeReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MakeReq) ProtoMessage() {}

func (x *MakeReq) ProtoReflect() protoreflect.Message {
	mi := &file_realm_proto_realm_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MakeReq.ProtoReflect.Descriptor instead.
func (*MakeReq) Descriptor() ([]byte, []int) {
	return file_realm_proto_realm_proto_rawDescGZIP(), []int{0}
}

func (x *MakeReq) GetRealm() string {
	if x != nil {
		return x.Realm
	}
	return ""
}

func (x *MakeReq) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *MakeReq) GetNumS3() int64 {
	if x != nil {
		return x.NumS3
	}
	return 0
}

func (x *MakeReq) GetNumUX() int64 {
	if x != nil {
		return x.NumUX
	}
	return 0
}

type MakeRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NamedAddr []string `protobuf:"bytes,1,rep,name=namedAddr,proto3" json:"namedAddr,omitempty"`
}

func (x *MakeRep) Reset() {
	*x = MakeRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_realm_proto_realm_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MakeRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MakeRep) ProtoMessage() {}

func (x *MakeRep) ProtoReflect() protoreflect.Message {
	mi := &file_realm_proto_realm_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MakeRep.ProtoReflect.Descriptor instead.
func (*MakeRep) Descriptor() ([]byte, []int) {
	return file_realm_proto_realm_proto_rawDescGZIP(), []int{1}
}

func (x *MakeRep) GetNamedAddr() []string {
	if x != nil {
		return x.NamedAddr
	}
	return nil
}

type RemoveReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Realm string `protobuf:"bytes,1,opt,name=realm,proto3" json:"realm,omitempty"`
}

func (x *RemoveReq) Reset() {
	*x = RemoveReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_realm_proto_realm_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RemoveReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RemoveReq) ProtoMessage() {}

func (x *RemoveReq) ProtoReflect() protoreflect.Message {
	mi := &file_realm_proto_realm_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RemoveReq.ProtoReflect.Descriptor instead.
func (*RemoveReq) Descriptor() ([]byte, []int) {
	return file_realm_proto_realm_proto_rawDescGZIP(), []int{2}
}

func (x *RemoveReq) GetRealm() string {
	if x != nil {
		return x.Realm
	}
	return ""
}

type RemoveRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *RemoveRep) Reset() {
	*x = RemoveRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_realm_proto_realm_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RemoveRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RemoveRep) ProtoMessage() {}

func (x *RemoveRep) ProtoReflect() protoreflect.Message {
	mi := &file_realm_proto_realm_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RemoveRep.ProtoReflect.Descriptor instead.
func (*RemoveRep) Descriptor() ([]byte, []int) {
	return file_realm_proto_realm_proto_rawDescGZIP(), []int{3}
}

var File_realm_proto_realm_proto protoreflect.FileDescriptor

var file_realm_proto_realm_proto_rawDesc = []byte{
	0x0a, 0x17, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x65,
	0x61, 0x6c, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x65, 0x0a, 0x07, 0x4d, 0x61, 0x6b,
	0x65, 0x52, 0x65, 0x71, 0x12, 0x14, 0x0a, 0x05, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x12, 0x18, 0x0a, 0x07, 0x6e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x12, 0x14, 0x0a, 0x05, 0x6e, 0x75, 0x6d, 0x53, 0x33, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x6e, 0x75, 0x6d, 0x53, 0x33, 0x12, 0x14, 0x0a, 0x05, 0x6e, 0x75,
	0x6d, 0x55, 0x58, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x6e, 0x75, 0x6d, 0x55, 0x58,
	0x22, 0x27, 0x0a, 0x07, 0x4d, 0x61, 0x6b, 0x65, 0x52, 0x65, 0x70, 0x12, 0x1c, 0x0a, 0x09, 0x6e,
	0x61, 0x6d, 0x65, 0x64, 0x41, 0x64, 0x64, 0x72, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09,
	0x6e, 0x61, 0x6d, 0x65, 0x64, 0x41, 0x64, 0x64, 0x72, 0x22, 0x21, 0x0a, 0x09, 0x52, 0x65, 0x6d,
	0x6f, 0x76, 0x65, 0x52, 0x65, 0x71, 0x12, 0x14, 0x0a, 0x05, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x22, 0x0b, 0x0a, 0x09,
	0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x52, 0x65, 0x70, 0x42, 0x15, 0x5a, 0x13, 0x73, 0x69, 0x67,
	0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_realm_proto_realm_proto_rawDescOnce sync.Once
	file_realm_proto_realm_proto_rawDescData = file_realm_proto_realm_proto_rawDesc
)

func file_realm_proto_realm_proto_rawDescGZIP() []byte {
	file_realm_proto_realm_proto_rawDescOnce.Do(func() {
		file_realm_proto_realm_proto_rawDescData = protoimpl.X.CompressGZIP(file_realm_proto_realm_proto_rawDescData)
	})
	return file_realm_proto_realm_proto_rawDescData
}

var file_realm_proto_realm_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_realm_proto_realm_proto_goTypes = []interface{}{
	(*MakeReq)(nil),   // 0: MakeReq
	(*MakeRep)(nil),   // 1: MakeRep
	(*RemoveReq)(nil), // 2: RemoveReq
	(*RemoveRep)(nil), // 3: RemoveRep
}
var file_realm_proto_realm_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_realm_proto_realm_proto_init() }
func file_realm_proto_realm_proto_init() {
	if File_realm_proto_realm_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_realm_proto_realm_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MakeReq); i {
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
		file_realm_proto_realm_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MakeRep); i {
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
		file_realm_proto_realm_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RemoveReq); i {
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
		file_realm_proto_realm_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RemoveRep); i {
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
			RawDescriptor: file_realm_proto_realm_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_realm_proto_realm_proto_goTypes,
		DependencyIndexes: file_realm_proto_realm_proto_depIdxs,
		MessageInfos:      file_realm_proto_realm_proto_msgTypes,
	}.Build()
	File_realm_proto_realm_proto = out.File
	file_realm_proto_realm_proto_rawDesc = nil
	file_realm_proto_realm_proto_goTypes = nil
	file_realm_proto_realm_proto_depIdxs = nil
}
