// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.15.8
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

type Tfenceid struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path     uint64 `protobuf:"varint,1,opt,name=path,proto3" json:"path,omitempty"`
	Serverid uint64 `protobuf:"varint,2,opt,name=serverid,proto3" json:"serverid,omitempty"`
}

func (x *Tfenceid) Reset() {
	*x = Tfenceid{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sessp_sessp_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Tfenceid) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Tfenceid) ProtoMessage() {}

func (x *Tfenceid) ProtoReflect() protoreflect.Message {
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

// Deprecated: Use Tfenceid.ProtoReflect.Descriptor instead.
func (*Tfenceid) Descriptor() ([]byte, []int) {
	return file_sessp_sessp_proto_rawDescGZIP(), []int{0}
}

func (x *Tfenceid) GetPath() uint64 {
	if x != nil {
		return x.Path
	}
	return 0
}

func (x *Tfenceid) GetServerid() uint64 {
	if x != nil {
		return x.Serverid
	}
	return 0
}

type Tfence struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fenceid *Tfenceid `protobuf:"bytes,1,opt,name=fenceid,proto3" json:"fenceid,omitempty"`
	Epoch   uint64    `protobuf:"varint,2,opt,name=epoch,proto3" json:"epoch,omitempty"`
}

func (x *Tfence) Reset() {
	*x = Tfence{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sessp_sessp_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Tfence) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Tfence) ProtoMessage() {}

func (x *Tfence) ProtoReflect() protoreflect.Message {
	mi := &file_sessp_sessp_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Tfence.ProtoReflect.Descriptor instead.
func (*Tfence) Descriptor() ([]byte, []int) {
	return file_sessp_sessp_proto_rawDescGZIP(), []int{1}
}

func (x *Tfence) GetFenceid() *Tfenceid {
	if x != nil {
		return x.Fenceid
	}
	return nil
}

func (x *Tfence) GetEpoch() uint64 {
	if x != nil {
		return x.Epoch
	}
	return 0
}

type Tinterval struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Start uint64 `protobuf:"varint,1,opt,name=start,proto3" json:"start,omitempty"`
	End   uint64 `protobuf:"varint,2,opt,name=end,proto3" json:"end,omitempty"`
}

func (x *Tinterval) Reset() {
	*x = Tinterval{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sessp_sessp_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Tinterval) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Tinterval) ProtoMessage() {}

func (x *Tinterval) ProtoReflect() protoreflect.Message {
	mi := &file_sessp_sessp_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Tinterval.ProtoReflect.Descriptor instead.
func (*Tinterval) Descriptor() ([]byte, []int) {
	return file_sessp_sessp_proto_rawDescGZIP(), []int{2}
}

func (x *Tinterval) GetStart() uint64 {
	if x != nil {
		return x.Start
	}
	return 0
}

func (x *Tinterval) GetEnd() uint64 {
	if x != nil {
		return x.End
	}
	return 0
}

type Fcall struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type     uint32     `protobuf:"varint,1,opt,name=type,proto3" json:"type,omitempty"`
	Tag      uint32     `protobuf:"varint,2,opt,name=tag,proto3" json:"tag,omitempty"`
	Client   uint64     `protobuf:"varint,3,opt,name=client,proto3" json:"client,omitempty"`
	Session  uint64     `protobuf:"varint,4,opt,name=session,proto3" json:"session,omitempty"`
	Seqno    uint64     `protobuf:"varint,5,opt,name=seqno,proto3" json:"seqno,omitempty"`
	Received *Tinterval `protobuf:"bytes,6,opt,name=received,proto3" json:"received,omitempty"`
	Fence    *Tfence    `protobuf:"bytes,7,opt,name=fence,proto3" json:"fence,omitempty"`
}

func (x *Fcall) Reset() {
	*x = Fcall{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sessp_sessp_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Fcall) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Fcall) ProtoMessage() {}

func (x *Fcall) ProtoReflect() protoreflect.Message {
	mi := &file_sessp_sessp_proto_msgTypes[3]
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
	return file_sessp_sessp_proto_rawDescGZIP(), []int{3}
}

func (x *Fcall) GetType() uint32 {
	if x != nil {
		return x.Type
	}
	return 0
}

func (x *Fcall) GetTag() uint32 {
	if x != nil {
		return x.Tag
	}
	return 0
}

func (x *Fcall) GetClient() uint64 {
	if x != nil {
		return x.Client
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

func (x *Fcall) GetReceived() *Tinterval {
	if x != nil {
		return x.Received
	}
	return nil
}

func (x *Fcall) GetFence() *Tfence {
	if x != nil {
		return x.Fence
	}
	return nil
}

var File_sessp_sessp_proto protoreflect.FileDescriptor

var file_sessp_sessp_proto_rawDesc = []byte{
	0x0a, 0x11, 0x73, 0x65, 0x73, 0x73, 0x70, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x70, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x3a, 0x0a, 0x08, 0x54, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x69, 0x64, 0x12,
	0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x70,
	0x61, 0x74, 0x68, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x69, 0x64, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x69, 0x64, 0x22,
	0x43, 0x0a, 0x06, 0x54, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x23, 0x0a, 0x07, 0x66, 0x65, 0x6e,
	0x63, 0x65, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x54, 0x66, 0x65,
	0x6e, 0x63, 0x65, 0x69, 0x64, 0x52, 0x07, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x69, 0x64, 0x12, 0x14,
	0x0a, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x65,
	0x70, 0x6f, 0x63, 0x68, 0x22, 0x33, 0x0a, 0x09, 0x54, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61,
	0x6c, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x05, 0x73, 0x74, 0x61, 0x72, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x65, 0x6e, 0x64, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x03, 0x65, 0x6e, 0x64, 0x22, 0xbc, 0x01, 0x0a, 0x05, 0x46, 0x63,
	0x61, 0x6c, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x74, 0x61, 0x67, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x03, 0x74, 0x61, 0x67, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x63, 0x6c, 0x69, 0x65, 0x6e,
	0x74, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x14, 0x0a, 0x05, 0x73,
	0x65, 0x71, 0x6e, 0x6f, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x73, 0x65, 0x71, 0x6e,
	0x6f, 0x12, 0x26, 0x0a, 0x08, 0x72, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x54, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61, 0x6c, 0x52,
	0x08, 0x72, 0x65, 0x63, 0x65, 0x69, 0x76, 0x65, 0x64, 0x12, 0x1d, 0x0a, 0x05, 0x66, 0x65, 0x6e,
	0x63, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x54, 0x66, 0x65, 0x6e, 0x63,
	0x65, 0x52, 0x05, 0x66, 0x65, 0x6e, 0x63, 0x65, 0x42, 0x0f, 0x5a, 0x0d, 0x73, 0x69, 0x67, 0x6d,
	0x61, 0x6f, 0x73, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x70, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
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

var file_sessp_sessp_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_sessp_sessp_proto_goTypes = []interface{}{
	(*Tfenceid)(nil),  // 0: Tfenceid
	(*Tfence)(nil),    // 1: Tfence
	(*Tinterval)(nil), // 2: Tinterval
	(*Fcall)(nil),     // 3: Fcall
}
var file_sessp_sessp_proto_depIdxs = []int32{
	0, // 0: Tfence.fenceid:type_name -> Tfenceid
	2, // 1: Fcall.received:type_name -> Tinterval
	1, // 2: Fcall.fence:type_name -> Tfence
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_sessp_sessp_proto_init() }
func file_sessp_sessp_proto_init() {
	if File_sessp_sessp_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sessp_sessp_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Tfenceid); i {
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
		file_sessp_sessp_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Tfence); i {
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
		file_sessp_sessp_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Tinterval); i {
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
		file_sessp_sessp_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
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
			NumMessages:   4,
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
