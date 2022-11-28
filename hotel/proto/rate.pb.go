// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.9
// source: hotel/proto/rate.proto

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

// The latitude and longitude of the current location.
type RateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HotelIds []string `protobuf:"bytes,1,rep,name=hotelIds,proto3" json:"hotelIds,omitempty"`
	InDate   string   `protobuf:"bytes,2,opt,name=inDate,proto3" json:"inDate,omitempty"`
	OutDate  string   `protobuf:"bytes,3,opt,name=outDate,proto3" json:"outDate,omitempty"`
}

func (x *RateRequest) Reset() {
	*x = RateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_rate_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RateRequest) ProtoMessage() {}

func (x *RateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_rate_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RateRequest.ProtoReflect.Descriptor instead.
func (*RateRequest) Descriptor() ([]byte, []int) {
	return file_hotel_proto_rate_proto_rawDescGZIP(), []int{0}
}

func (x *RateRequest) GetHotelIds() []string {
	if x != nil {
		return x.HotelIds
	}
	return nil
}

func (x *RateRequest) GetInDate() string {
	if x != nil {
		return x.InDate
	}
	return ""
}

func (x *RateRequest) GetOutDate() string {
	if x != nil {
		return x.OutDate
	}
	return ""
}

type RoomType struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BookableRate       float64 `protobuf:"fixed64,1,opt,name=bookableRate,proto3" json:"bookableRate,omitempty"`
	TotalRate          float64 `protobuf:"fixed64,2,opt,name=totalRate,proto3" json:"totalRate,omitempty"`
	TotalRateInclusive float64 `protobuf:"fixed64,3,opt,name=totalRateInclusive,proto3" json:"totalRateInclusive,omitempty"`
	Code               string  `protobuf:"bytes,4,opt,name=code,proto3" json:"code,omitempty"`
	Currency           string  `protobuf:"bytes,5,opt,name=currency,proto3" json:"currency,omitempty"`
	RoomDescription    string  `protobuf:"bytes,6,opt,name=roomDescription,proto3" json:"roomDescription,omitempty"`
}

func (x *RoomType) Reset() {
	*x = RoomType{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_rate_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RoomType) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RoomType) ProtoMessage() {}

func (x *RoomType) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_rate_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RoomType.ProtoReflect.Descriptor instead.
func (*RoomType) Descriptor() ([]byte, []int) {
	return file_hotel_proto_rate_proto_rawDescGZIP(), []int{1}
}

func (x *RoomType) GetBookableRate() float64 {
	if x != nil {
		return x.BookableRate
	}
	return 0
}

func (x *RoomType) GetTotalRate() float64 {
	if x != nil {
		return x.TotalRate
	}
	return 0
}

func (x *RoomType) GetTotalRateInclusive() float64 {
	if x != nil {
		return x.TotalRateInclusive
	}
	return 0
}

func (x *RoomType) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *RoomType) GetCurrency() string {
	if x != nil {
		return x.Currency
	}
	return ""
}

func (x *RoomType) GetRoomDescription() string {
	if x != nil {
		return x.RoomDescription
	}
	return ""
}

type RatePlan struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HotelId  string    `protobuf:"bytes,1,opt,name=hotelId,proto3" json:"hotelId,omitempty"`
	Code     string    `protobuf:"bytes,2,opt,name=code,proto3" json:"code,omitempty"`
	InDate   string    `protobuf:"bytes,3,opt,name=inDate,proto3" json:"inDate,omitempty"`
	OutDate  string    `protobuf:"bytes,4,opt,name=outDate,proto3" json:"outDate,omitempty"`
	RoomType *RoomType `protobuf:"bytes,5,opt,name=roomType,proto3" json:"roomType,omitempty"`
}

func (x *RatePlan) Reset() {
	*x = RatePlan{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_rate_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RatePlan) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RatePlan) ProtoMessage() {}

func (x *RatePlan) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_rate_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RatePlan.ProtoReflect.Descriptor instead.
func (*RatePlan) Descriptor() ([]byte, []int) {
	return file_hotel_proto_rate_proto_rawDescGZIP(), []int{2}
}

func (x *RatePlan) GetHotelId() string {
	if x != nil {
		return x.HotelId
	}
	return ""
}

func (x *RatePlan) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *RatePlan) GetInDate() string {
	if x != nil {
		return x.InDate
	}
	return ""
}

func (x *RatePlan) GetOutDate() string {
	if x != nil {
		return x.OutDate
	}
	return ""
}

func (x *RatePlan) GetRoomType() *RoomType {
	if x != nil {
		return x.RoomType
	}
	return nil
}

type RateResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RatePlans []*RatePlan `protobuf:"bytes,1,rep,name=ratePlans,proto3" json:"ratePlans,omitempty"`
}

func (x *RateResult) Reset() {
	*x = RateResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hotel_proto_rate_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RateResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RateResult) ProtoMessage() {}

func (x *RateResult) ProtoReflect() protoreflect.Message {
	mi := &file_hotel_proto_rate_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RateResult.ProtoReflect.Descriptor instead.
func (*RateResult) Descriptor() ([]byte, []int) {
	return file_hotel_proto_rate_proto_rawDescGZIP(), []int{3}
}

func (x *RateResult) GetRatePlans() []*RatePlan {
	if x != nil {
		return x.RatePlans
	}
	return nil
}

var File_hotel_proto_rate_proto protoreflect.FileDescriptor

var file_hotel_proto_rate_proto_rawDesc = []byte{
	0x0a, 0x16, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x61,
	0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5b, 0x0a, 0x0b, 0x52, 0x61, 0x74, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x68, 0x6f, 0x74, 0x65, 0x6c,
	0x49, 0x64, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x74, 0x65, 0x6c,
	0x49, 0x64, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x6e, 0x44, 0x61, 0x74, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x69, 0x6e, 0x44, 0x61, 0x74, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6f,
	0x75, 0x74, 0x44, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6f, 0x75,
	0x74, 0x44, 0x61, 0x74, 0x65, 0x22, 0xd6, 0x01, 0x0a, 0x08, 0x52, 0x6f, 0x6f, 0x6d, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x22, 0x0a, 0x0c, 0x62, 0x6f, 0x6f, 0x6b, 0x61, 0x62, 0x6c, 0x65, 0x52, 0x61,
	0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0c, 0x62, 0x6f, 0x6f, 0x6b, 0x61, 0x62,
	0x6c, 0x65, 0x52, 0x61, 0x74, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x52,
	0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x01, 0x52, 0x09, 0x74, 0x6f, 0x74, 0x61, 0x6c,
	0x52, 0x61, 0x74, 0x65, 0x12, 0x2e, 0x0a, 0x12, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x52, 0x61, 0x74,
	0x65, 0x49, 0x6e, 0x63, 0x6c, 0x75, 0x73, 0x69, 0x76, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x12, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x52, 0x61, 0x74, 0x65, 0x49, 0x6e, 0x63, 0x6c, 0x75,
	0x73, 0x69, 0x76, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x63, 0x75, 0x72, 0x72,
	0x65, 0x6e, 0x63, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x75, 0x72, 0x72,
	0x65, 0x6e, 0x63, 0x79, 0x12, 0x28, 0x0a, 0x0f, 0x72, 0x6f, 0x6f, 0x6d, 0x44, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x72,
	0x6f, 0x6f, 0x6d, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x91,
	0x01, 0x0a, 0x08, 0x52, 0x61, 0x74, 0x65, 0x50, 0x6c, 0x61, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x68,
	0x6f, 0x74, 0x65, 0x6c, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x68, 0x6f,
	0x74, 0x65, 0x6c, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x6e, 0x44,
	0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x69, 0x6e, 0x44, 0x61, 0x74,
	0x65, 0x12, 0x18, 0x0a, 0x07, 0x6f, 0x75, 0x74, 0x44, 0x61, 0x74, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x6f, 0x75, 0x74, 0x44, 0x61, 0x74, 0x65, 0x12, 0x25, 0x0a, 0x08, 0x72,
	0x6f, 0x6f, 0x6d, 0x54, 0x79, 0x70, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x09, 0x2e,
	0x52, 0x6f, 0x6f, 0x6d, 0x54, 0x79, 0x70, 0x65, 0x52, 0x08, 0x72, 0x6f, 0x6f, 0x6d, 0x54, 0x79,
	0x70, 0x65, 0x22, 0x35, 0x0a, 0x0a, 0x52, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74,
	0x12, 0x27, 0x0a, 0x09, 0x72, 0x61, 0x74, 0x65, 0x50, 0x6c, 0x61, 0x6e, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x52, 0x61, 0x74, 0x65, 0x50, 0x6c, 0x61, 0x6e, 0x52, 0x09,
	0x72, 0x61, 0x74, 0x65, 0x50, 0x6c, 0x61, 0x6e, 0x73, 0x32, 0x2d, 0x0a, 0x04, 0x52, 0x61, 0x74,
	0x65, 0x12, 0x25, 0x0a, 0x08, 0x47, 0x65, 0x74, 0x52, 0x61, 0x74, 0x65, 0x73, 0x12, 0x0c, 0x2e,
	0x52, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0b, 0x2e, 0x52, 0x61,
	0x74, 0x65, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x42, 0x15, 0x5a, 0x13, 0x75, 0x6c, 0x61, 0x6d,
	0x62, 0x64, 0x61, 0x2f, 0x68, 0x6f, 0x74, 0x65, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_hotel_proto_rate_proto_rawDescOnce sync.Once
	file_hotel_proto_rate_proto_rawDescData = file_hotel_proto_rate_proto_rawDesc
)

func file_hotel_proto_rate_proto_rawDescGZIP() []byte {
	file_hotel_proto_rate_proto_rawDescOnce.Do(func() {
		file_hotel_proto_rate_proto_rawDescData = protoimpl.X.CompressGZIP(file_hotel_proto_rate_proto_rawDescData)
	})
	return file_hotel_proto_rate_proto_rawDescData
}

var file_hotel_proto_rate_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_hotel_proto_rate_proto_goTypes = []interface{}{
	(*RateRequest)(nil), // 0: RateRequest
	(*RoomType)(nil),    // 1: RoomType
	(*RatePlan)(nil),    // 2: RatePlan
	(*RateResult)(nil),  // 3: RateResult
}
var file_hotel_proto_rate_proto_depIdxs = []int32{
	1, // 0: RatePlan.roomType:type_name -> RoomType
	2, // 1: RateResult.ratePlans:type_name -> RatePlan
	0, // 2: Rate.GetRates:input_type -> RateRequest
	3, // 3: Rate.GetRates:output_type -> RateResult
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_hotel_proto_rate_proto_init() }
func file_hotel_proto_rate_proto_init() {
	if File_hotel_proto_rate_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_hotel_proto_rate_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RateRequest); i {
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
		file_hotel_proto_rate_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RoomType); i {
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
		file_hotel_proto_rate_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RatePlan); i {
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
		file_hotel_proto_rate_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RateResult); i {
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
			RawDescriptor: file_hotel_proto_rate_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_hotel_proto_rate_proto_goTypes,
		DependencyIndexes: file_hotel_proto_rate_proto_depIdxs,
		MessageInfos:      file_hotel_proto_rate_proto_msgTypes,
	}.Build()
	File_hotel_proto_rate_proto = out.File
	file_hotel_proto_rate_proto_rawDesc = nil
	file_hotel_proto_rate_proto_goTypes = nil
	file_hotel_proto_rate_proto_depIdxs = nil
}
