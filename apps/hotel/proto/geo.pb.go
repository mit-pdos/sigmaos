// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: apps/hotel/proto/geo.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	proto "sigmaos/util/tracing/proto"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GeoReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Lat               float32                  `protobuf:"fixed32,1,opt,name=lat,proto3" json:"lat,omitempty"`
	Lon               float32                  `protobuf:"fixed32,2,opt,name=lon,proto3" json:"lon,omitempty"`
	SpanContextConfig *proto.SpanContextConfig `protobuf:"bytes,3,opt,name=spanContextConfig,proto3" json:"spanContextConfig,omitempty"`
}

func (x *GeoReq) Reset() {
	*x = GeoReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_hotel_proto_geo_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GeoReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GeoReq) ProtoMessage() {}

func (x *GeoReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_hotel_proto_geo_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GeoReq.ProtoReflect.Descriptor instead.
func (*GeoReq) Descriptor() ([]byte, []int) {
	return file_apps_hotel_proto_geo_proto_rawDescGZIP(), []int{0}
}

func (x *GeoReq) GetLat() float32 {
	if x != nil {
		return x.Lat
	}
	return 0
}

func (x *GeoReq) GetLon() float32 {
	if x != nil {
		return x.Lon
	}
	return 0
}

func (x *GeoReq) GetSpanContextConfig() *proto.SpanContextConfig {
	if x != nil {
		return x.SpanContextConfig
	}
	return nil
}

type GeoRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HotelIds []string `protobuf:"bytes,1,rep,name=hotelIds,proto3" json:"hotelIds,omitempty"`
}

func (x *GeoRep) Reset() {
	*x = GeoRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_hotel_proto_geo_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GeoRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GeoRep) ProtoMessage() {}

func (x *GeoRep) ProtoReflect() protoreflect.Message {
	mi := &file_apps_hotel_proto_geo_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GeoRep.ProtoReflect.Descriptor instead.
func (*GeoRep) Descriptor() ([]byte, []int) {
	return file_apps_hotel_proto_geo_proto_rawDescGZIP(), []int{1}
}

func (x *GeoRep) GetHotelIds() []string {
	if x != nil {
		return x.HotelIds
	}
	return nil
}

var File_apps_hotel_proto_geo_proto protoreflect.FileDescriptor

var file_apps_hotel_proto_geo_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x61, 0x70, 0x70, 0x73, 0x2f, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x67, 0x65, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x20, 0x75, 0x74,
	0x69, 0x6c, 0x2f, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x6e,
	0x0a, 0x06, 0x47, 0x65, 0x6f, 0x52, 0x65, 0x71, 0x12, 0x10, 0x0a, 0x03, 0x6c, 0x61, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x02, 0x52, 0x03, 0x6c, 0x61, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6c, 0x6f,
	0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x02, 0x52, 0x03, 0x6c, 0x6f, 0x6e, 0x12, 0x40, 0x0a, 0x11,
	0x73, 0x70, 0x61, 0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x53, 0x70, 0x61, 0x6e, 0x43, 0x6f,
	0x6e, 0x74, 0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x11, 0x73, 0x70, 0x61,
	0x6e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x22, 0x24,
	0x0a, 0x06, 0x47, 0x65, 0x6f, 0x52, 0x65, 0x70, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x74, 0x65,
	0x6c, 0x49, 0x64, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x74, 0x65,
	0x6c, 0x49, 0x64, 0x73, 0x32, 0x21, 0x0a, 0x03, 0x47, 0x65, 0x6f, 0x12, 0x1a, 0x0a, 0x06, 0x4e,
	0x65, 0x61, 0x72, 0x62, 0x79, 0x12, 0x07, 0x2e, 0x47, 0x65, 0x6f, 0x52, 0x65, 0x71, 0x1a, 0x07,
	0x2e, 0x47, 0x65, 0x6f, 0x52, 0x65, 0x70, 0x42, 0x1a, 0x5a, 0x18, 0x73, 0x69, 0x67, 0x6d, 0x61,
	0x6f, 0x73, 0x2f, 0x61, 0x70, 0x70, 0x73, 0x2f, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_apps_hotel_proto_geo_proto_rawDescOnce sync.Once
	file_apps_hotel_proto_geo_proto_rawDescData = file_apps_hotel_proto_geo_proto_rawDesc
)

func file_apps_hotel_proto_geo_proto_rawDescGZIP() []byte {
	file_apps_hotel_proto_geo_proto_rawDescOnce.Do(func() {
		file_apps_hotel_proto_geo_proto_rawDescData = protoimpl.X.CompressGZIP(file_apps_hotel_proto_geo_proto_rawDescData)
	})
	return file_apps_hotel_proto_geo_proto_rawDescData
}

var file_apps_hotel_proto_geo_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_apps_hotel_proto_geo_proto_goTypes = []interface{}{
	(*GeoReq)(nil),                  // 0: GeoReq
	(*GeoRep)(nil),                  // 1: GeoRep
	(*proto.SpanContextConfig)(nil), // 2: SpanContextConfig
}
var file_apps_hotel_proto_geo_proto_depIdxs = []int32{
	2, // 0: GeoReq.spanContextConfig:type_name -> SpanContextConfig
	0, // 1: Geo.Nearby:input_type -> GeoReq
	1, // 2: Geo.Nearby:output_type -> GeoRep
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_apps_hotel_proto_geo_proto_init() }
func file_apps_hotel_proto_geo_proto_init() {
	if File_apps_hotel_proto_geo_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_apps_hotel_proto_geo_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GeoReq); i {
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
		file_apps_hotel_proto_geo_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GeoRep); i {
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
			RawDescriptor: file_apps_hotel_proto_geo_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_apps_hotel_proto_geo_proto_goTypes,
		DependencyIndexes: file_apps_hotel_proto_geo_proto_depIdxs,
		MessageInfos:      file_apps_hotel_proto_geo_proto_msgTypes,
	}.Build()
	File_apps_hotel_proto_geo_proto = out.File
	file_apps_hotel_proto_geo_proto_rawDesc = nil
	file_apps_hotel_proto_geo_proto_goTypes = nil
	file_apps_hotel_proto_geo_proto_depIdxs = nil
}
