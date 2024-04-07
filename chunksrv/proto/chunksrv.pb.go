// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.25.3
// source: chunksrv/proto/chunksrv.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proto "sigmaos/rpc/proto"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type FetchChunkRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Prog    string `protobuf:"bytes,1,opt,name=prog,proto3" json:"prog,omitempty"`
	ChunkId int32  `protobuf:"varint,2,opt,name=chunkId,proto3" json:"chunkId,omitempty"`
	Size    uint64 `protobuf:"varint,3,opt,name=size,proto3" json:"size,omitempty"`
	Realm   string `protobuf:"bytes,4,opt,name=realm,proto3" json:"realm,omitempty"`
}

func (x *FetchChunkRequest) Reset() {
	*x = FetchChunkRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chunksrv_proto_chunksrv_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchChunkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchChunkRequest) ProtoMessage() {}

func (x *FetchChunkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_chunksrv_proto_chunksrv_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchChunkRequest.ProtoReflect.Descriptor instead.
func (*FetchChunkRequest) Descriptor() ([]byte, []int) {
	return file_chunksrv_proto_chunksrv_proto_rawDescGZIP(), []int{0}
}

func (x *FetchChunkRequest) GetProg() string {
	if x != nil {
		return x.Prog
	}
	return ""
}

func (x *FetchChunkRequest) GetChunkId() int32 {
	if x != nil {
		return x.ChunkId
	}
	return 0
}

func (x *FetchChunkRequest) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *FetchChunkRequest) GetRealm() string {
	if x != nil {
		return x.Realm
	}
	return ""
}

type FetchChunkResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Blob *proto.Blob `protobuf:"bytes,1,opt,name=blob,proto3" json:"blob,omitempty"`
	Size uint64      `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
}

func (x *FetchChunkResponse) Reset() {
	*x = FetchChunkResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chunksrv_proto_chunksrv_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FetchChunkResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FetchChunkResponse) ProtoMessage() {}

func (x *FetchChunkResponse) ProtoReflect() protoreflect.Message {
	mi := &file_chunksrv_proto_chunksrv_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FetchChunkResponse.ProtoReflect.Descriptor instead.
func (*FetchChunkResponse) Descriptor() ([]byte, []int) {
	return file_chunksrv_proto_chunksrv_proto_rawDescGZIP(), []int{1}
}

func (x *FetchChunkResponse) GetBlob() *proto.Blob {
	if x != nil {
		return x.Blob
	}
	return nil
}

func (x *FetchChunkResponse) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

var File_chunksrv_proto_chunksrv_proto protoreflect.FileDescriptor

var file_chunksrv_proto_chunksrv_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x72, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x72, 0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x13, 0x72, 0x70, 0x63, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x70, 0x63, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x6b, 0x0a, 0x11, 0x46, 0x65, 0x74, 0x63, 0x68, 0x43, 0x68, 0x75,
	0x6e, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x72, 0x6f,
	0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x72, 0x6f, 0x67, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x49, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07,
	0x63, 0x68, 0x75, 0x6e, 0x6b, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x72,
	0x65, 0x61, 0x6c, 0x6d, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x65, 0x61, 0x6c,
	0x6d, 0x22, 0x43, 0x0a, 0x12, 0x46, 0x65, 0x74, 0x63, 0x68, 0x43, 0x68, 0x75, 0x6e, 0x6b, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x04, 0x62, 0x6c, 0x6f, 0x62, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x62, 0x6c,
	0x6f, 0x62, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x42, 0x18, 0x5a, 0x16, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f,
	0x73, 0x2f, 0x63, 0x68, 0x75, 0x6e, 0x6b, 0x73, 0x72, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_chunksrv_proto_chunksrv_proto_rawDescOnce sync.Once
	file_chunksrv_proto_chunksrv_proto_rawDescData = file_chunksrv_proto_chunksrv_proto_rawDesc
)

func file_chunksrv_proto_chunksrv_proto_rawDescGZIP() []byte {
	file_chunksrv_proto_chunksrv_proto_rawDescOnce.Do(func() {
		file_chunksrv_proto_chunksrv_proto_rawDescData = protoimpl.X.CompressGZIP(file_chunksrv_proto_chunksrv_proto_rawDescData)
	})
	return file_chunksrv_proto_chunksrv_proto_rawDescData
}

var file_chunksrv_proto_chunksrv_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_chunksrv_proto_chunksrv_proto_goTypes = []interface{}{
	(*FetchChunkRequest)(nil),  // 0: FetchChunkRequest
	(*FetchChunkResponse)(nil), // 1: FetchChunkResponse
	(*proto.Blob)(nil),         // 2: Blob
}
var file_chunksrv_proto_chunksrv_proto_depIdxs = []int32{
	2, // 0: FetchChunkResponse.blob:type_name -> Blob
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_chunksrv_proto_chunksrv_proto_init() }
func file_chunksrv_proto_chunksrv_proto_init() {
	if File_chunksrv_proto_chunksrv_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_chunksrv_proto_chunksrv_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchChunkRequest); i {
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
		file_chunksrv_proto_chunksrv_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FetchChunkResponse); i {
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
			RawDescriptor: file_chunksrv_proto_chunksrv_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_chunksrv_proto_chunksrv_proto_goTypes,
		DependencyIndexes: file_chunksrv_proto_chunksrv_proto_depIdxs,
		MessageInfos:      file_chunksrv_proto_chunksrv_proto_msgTypes,
	}.Build()
	File_chunksrv_proto_chunksrv_proto = out.File
	file_chunksrv_proto_chunksrv_proto_rawDesc = nil
	file_chunksrv_proto_chunksrv_proto_goTypes = nil
	file_chunksrv_proto_chunksrv_proto_depIdxs = nil
}
