// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.25.2
// source: sessp/sessp.proto

package sessp

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

type Fcall struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type    uint32 `protobuf:"varint,1,opt,name=type,proto3" json:"type,omitempty"`
	Session uint64 `protobuf:"varint,2,opt,name=session,proto3" json:"session,omitempty"`
	Seqno   uint64 `protobuf:"varint,3,opt,name=seqno,proto3" json:"seqno,omitempty"`
}

func (x *Fcall) Reset() {
	*x = Fcall{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sessp_sessp_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Fcall) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fcall) ProtoMessage() {}

func (x *Fcall) ProtoReflect() protoreflect.Message {
	mi := &file_sessp_sessp_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Fcall.ProtoReflect.Descriptor instead.
func (*Fcall) Descriptor() ([]byte, []int) {
	return file_sessp_sessp_proto_rawDescGZIP(), []int{0}
}

func (x *Fcall) GetType() uint32 {
	if x != nil {
		return x.Type
	}
	return 0
}

func (x *Fcall) GetSession() uint64 {
	if x != nil {
		return x.Session
	}
	return 0
}

func (x *Fcall) GetSeqno() uint64 {
	if x != nil {
		return x.Seqno
	}
	return 0
}

var File_sessp_sessp_proto protoreflect.FileDescriptor

var file_sessp_sessp_proto_rawDesc = []byte{
	0x0a, 0x11, 0x73, 0x65, 0x73, 0x73, 0x70, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x70, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x4b, 0x0a, 0x05, 0x46, 0x63, 0x61, 0x6c, 0x6c, 0x12, 0x12, 0x0a, 0x04,
	0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x65,
	0x71, 0x6e, 0x6f, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x73, 0x65, 0x71, 0x6e, 0x6f,
	0x42, 0x0f, 0x5a, 0x0d, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x73, 0x65, 0x73, 0x73,
	0x70, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sessp_sessp_proto_rawDescOnce sync.Once
	file_sessp_sessp_proto_rawDescData = file_sessp_sessp_proto_rawDesc
)

func file_sessp_sessp_proto_rawDescGZIP() []byte {
	file_sessp_sessp_proto_rawDescOnce.Do(func() {
		file_sessp_sessp_proto_rawDescData = protoimpl.X.CompressGZIP(file_sessp_sessp_proto_rawDescData)
	})
	return file_sessp_sessp_proto_rawDescData
}

var file_sessp_sessp_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_sessp_sessp_proto_goTypes = []interface{}{
	(*Fcall)(nil), // 0: Fcall
}
var file_sessp_sessp_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_sessp_sessp_proto_init() }
func file_sessp_sessp_proto_init() {
	if File_sessp_sessp_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sessp_sessp_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Fcall); i {
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
			RawDescriptor: file_sessp_sessp_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sessp_sessp_proto_goTypes,
		DependencyIndexes: file_sessp_sessp_proto_depIdxs,
		MessageInfos:      file_sessp_sessp_proto_msgTypes,
	}.Build()
	File_sessp_sessp_proto = out.File
	file_sessp_sessp_proto_rawDesc = nil
	file_sessp_sessp_proto_goTypes = nil
	file_sessp_sessp_proto_depIdxs = nil
}
