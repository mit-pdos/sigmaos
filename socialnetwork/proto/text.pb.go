// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.15.8
// source: socialnetwork/proto/text.proto

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

type ComposeUrlsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Extendedurls []string `protobuf:"bytes,1,rep,name=extendedurls,proto3" json:"extendedurls,omitempty"`
}

func (x *ComposeUrlsRequest) Reset() {
	*x = ComposeUrlsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ComposeUrlsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ComposeUrlsRequest) ProtoMessage() {}

func (x *ComposeUrlsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ComposeUrlsRequest.ProtoReflect.Descriptor instead.
func (*ComposeUrlsRequest) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{0}
}

func (x *ComposeUrlsRequest) GetExtendedurls() []string {
	if x != nil {
		return x.Extendedurls
	}
	return nil
}

type ComposeUrlsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok   string `protobuf:"bytes,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Urls []*Url `protobuf:"bytes,2,rep,name=urls,proto3" json:"urls,omitempty"`
}

func (x *ComposeUrlsResponse) Reset() {
	*x = ComposeUrlsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ComposeUrlsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ComposeUrlsResponse) ProtoMessage() {}

func (x *ComposeUrlsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ComposeUrlsResponse.ProtoReflect.Descriptor instead.
func (*ComposeUrlsResponse) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{1}
}

func (x *ComposeUrlsResponse) GetOk() string {
	if x != nil {
		return x.Ok
	}
	return ""
}

func (x *ComposeUrlsResponse) GetUrls() []*Url {
	if x != nil {
		return x.Urls
	}
	return nil
}

type GetUrlsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Shorturls []string `protobuf:"bytes,1,rep,name=shorturls,proto3" json:"shorturls,omitempty"`
}

func (x *GetUrlsRequest) Reset() {
	*x = GetUrlsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetUrlsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUrlsRequest) ProtoMessage() {}

func (x *GetUrlsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetUrlsRequest.ProtoReflect.Descriptor instead.
func (*GetUrlsRequest) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{2}
}

func (x *GetUrlsRequest) GetShorturls() []string {
	if x != nil {
		return x.Shorturls
	}
	return nil
}

type GetUrlsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok           string   `protobuf:"bytes,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Extendedurls []string `protobuf:"bytes,2,rep,name=extendedurls,proto3" json:"extendedurls,omitempty"`
}

func (x *GetUrlsResponse) Reset() {
	*x = GetUrlsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetUrlsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUrlsResponse) ProtoMessage() {}

func (x *GetUrlsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetUrlsResponse.ProtoReflect.Descriptor instead.
func (*GetUrlsResponse) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{3}
}

func (x *GetUrlsResponse) GetOk() string {
	if x != nil {
		return x.Ok
	}
	return ""
}

func (x *GetUrlsResponse) GetExtendedurls() []string {
	if x != nil {
		return x.Extendedurls
	}
	return nil
}

type ProcessTextRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Text string `protobuf:"bytes,1,opt,name=text,proto3" json:"text,omitempty"`
}

func (x *ProcessTextRequest) Reset() {
	*x = ProcessTextRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProcessTextRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProcessTextRequest) ProtoMessage() {}

func (x *ProcessTextRequest) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProcessTextRequest.ProtoReflect.Descriptor instead.
func (*ProcessTextRequest) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{4}
}

func (x *ProcessTextRequest) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

type ProcessTextResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok           string     `protobuf:"bytes,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Text         string     `protobuf:"bytes,2,opt,name=text,proto3" json:"text,omitempty"`
	Usermentions []*UserRef `protobuf:"bytes,3,rep,name=usermentions,proto3" json:"usermentions,omitempty"`
	Urls         []*Url     `protobuf:"bytes,4,rep,name=urls,proto3" json:"urls,omitempty"`
}

func (x *ProcessTextResponse) Reset() {
	*x = ProcessTextResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProcessTextResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProcessTextResponse) ProtoMessage() {}

func (x *ProcessTextResponse) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProcessTextResponse.ProtoReflect.Descriptor instead.
func (*ProcessTextResponse) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{5}
}

func (x *ProcessTextResponse) GetOk() string {
	if x != nil {
		return x.Ok
	}
	return ""
}

func (x *ProcessTextResponse) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (x *ProcessTextResponse) GetUsermentions() []*UserRef {
	if x != nil {
		return x.Usermentions
	}
	return nil
}

func (x *ProcessTextResponse) GetUrls() []*Url {
	if x != nil {
		return x.Urls
	}
	return nil
}

type UserRef struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Userid   int64  `protobuf:"varint,1,opt,name=userid,proto3" json:"userid,omitempty"`
	Username string `protobuf:"bytes,2,opt,name=username,proto3" json:"username,omitempty"`
}

func (x *UserRef) Reset() {
	*x = UserRef{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserRef) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserRef) ProtoMessage() {}

func (x *UserRef) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserRef.ProtoReflect.Descriptor instead.
func (*UserRef) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{6}
}

func (x *UserRef) GetUserid() int64 {
	if x != nil {
		return x.Userid
	}
	return 0
}

func (x *UserRef) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

type MediaRef struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Mediaid   int64  `protobuf:"varint,1,opt,name=mediaid,proto3" json:"mediaid,omitempty"`
	Mediatype string `protobuf:"bytes,2,opt,name=mediatype,proto3" json:"mediatype,omitempty"`
}

func (x *MediaRef) Reset() {
	*x = MediaRef{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MediaRef) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MediaRef) ProtoMessage() {}

func (x *MediaRef) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MediaRef.ProtoReflect.Descriptor instead.
func (*MediaRef) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{7}
}

func (x *MediaRef) GetMediaid() int64 {
	if x != nil {
		return x.Mediaid
	}
	return 0
}

func (x *MediaRef) GetMediatype() string {
	if x != nil {
		return x.Mediatype
	}
	return ""
}

type Url struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Shorturl    string `protobuf:"bytes,1,opt,name=shorturl,proto3" json:"shorturl,omitempty"`
	Extendedurl string `protobuf:"bytes,2,opt,name=extendedurl,proto3" json:"extendedurl,omitempty"`
}

func (x *Url) Reset() {
	*x = Url{}
	if protoimpl.UnsafeEnabled {
		mi := &file_socialnetwork_proto_text_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Url) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Url) ProtoMessage() {}

func (x *Url) ProtoReflect() protoreflect.Message {
	mi := &file_socialnetwork_proto_text_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Url.ProtoReflect.Descriptor instead.
func (*Url) Descriptor() ([]byte, []int) {
	return file_socialnetwork_proto_text_proto_rawDescGZIP(), []int{8}
}

func (x *Url) GetShorturl() string {
	if x != nil {
		return x.Shorturl
	}
	return ""
}

func (x *Url) GetExtendedurl() string {
	if x != nil {
		return x.Extendedurl
	}
	return ""
}

var File_socialnetwork_proto_text_proto protoreflect.FileDescriptor

var file_socialnetwork_proto_text_proto_rawDesc = []byte{
	0x0a, 0x1e, 0x73, 0x6f, 0x63, 0x69, 0x61, 0x6c, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x74, 0x65, 0x78, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x38, 0x0a, 0x12, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x73, 0x65, 0x55, 0x72, 0x6c, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x22, 0x0a, 0x0c, 0x65, 0x78, 0x74, 0x65, 0x6e, 0x64,
	0x65, 0x64, 0x75, 0x72, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0c, 0x65, 0x78,
	0x74, 0x65, 0x6e, 0x64, 0x65, 0x64, 0x75, 0x72, 0x6c, 0x73, 0x22, 0x3f, 0x0a, 0x13, 0x43, 0x6f,
	0x6d, 0x70, 0x6f, 0x73, 0x65, 0x55, 0x72, 0x6c, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x6f,
	0x6b, 0x12, 0x18, 0x0a, 0x04, 0x75, 0x72, 0x6c, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x04, 0x2e, 0x55, 0x72, 0x6c, 0x52, 0x04, 0x75, 0x72, 0x6c, 0x73, 0x22, 0x2e, 0x0a, 0x0e, 0x47,
	0x65, 0x74, 0x55, 0x72, 0x6c, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a,
	0x09, 0x73, 0x68, 0x6f, 0x72, 0x74, 0x75, 0x72, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x09, 0x73, 0x68, 0x6f, 0x72, 0x74, 0x75, 0x72, 0x6c, 0x73, 0x22, 0x45, 0x0a, 0x0f, 0x47,
	0x65, 0x74, 0x55, 0x72, 0x6c, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e,
	0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x6f, 0x6b, 0x12, 0x22,
	0x0a, 0x0c, 0x65, 0x78, 0x74, 0x65, 0x6e, 0x64, 0x65, 0x64, 0x75, 0x72, 0x6c, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x0c, 0x65, 0x78, 0x74, 0x65, 0x6e, 0x64, 0x65, 0x64, 0x75, 0x72,
	0x6c, 0x73, 0x22, 0x28, 0x0a, 0x12, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x54, 0x65, 0x78,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x65, 0x78, 0x74, 0x22, 0x81, 0x01, 0x0a,
	0x13, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x54, 0x65, 0x78, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x6f, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x74, 0x65, 0x78, 0x74, 0x12, 0x2c, 0x0a, 0x0c, 0x75, 0x73, 0x65, 0x72,
	0x6d, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x08,
	0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x66, 0x52, 0x0c, 0x75, 0x73, 0x65, 0x72, 0x6d, 0x65,
	0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x18, 0x0a, 0x04, 0x75, 0x72, 0x6c, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x04, 0x2e, 0x55, 0x72, 0x6c, 0x52, 0x04, 0x75, 0x72, 0x6c, 0x73,
	0x22, 0x3d, 0x0a, 0x07, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x66, 0x12, 0x16, 0x0a, 0x06, 0x75,
	0x73, 0x65, 0x72, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x75, 0x73, 0x65,
	0x72, 0x69, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x22,
	0x42, 0x0a, 0x08, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x52, 0x65, 0x66, 0x12, 0x18, 0x0a, 0x07, 0x6d,
	0x65, 0x64, 0x69, 0x61, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x6d, 0x65,
	0x64, 0x69, 0x61, 0x69, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x74,
	0x79, 0x70, 0x65, 0x22, 0x43, 0x0a, 0x03, 0x55, 0x72, 0x6c, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x68,
	0x6f, 0x72, 0x74, 0x75, 0x72, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x73, 0x68,
	0x6f, 0x72, 0x74, 0x75, 0x72, 0x6c, 0x12, 0x20, 0x0a, 0x0b, 0x65, 0x78, 0x74, 0x65, 0x6e, 0x64,
	0x65, 0x64, 0x75, 0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x65, 0x78, 0x74,
	0x65, 0x6e, 0x64, 0x65, 0x64, 0x75, 0x72, 0x6c, 0x32, 0x47, 0x0a, 0x0b, 0x54, 0x65, 0x78, 0x74,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x38, 0x0a, 0x0b, 0x50, 0x72, 0x6f, 0x63, 0x65,
	0x73, 0x73, 0x54, 0x65, 0x78, 0x74, 0x12, 0x13, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73,
	0x54, 0x65, 0x78, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x50, 0x72,
	0x6f, 0x63, 0x65, 0x73, 0x73, 0x54, 0x65, 0x78, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x32, 0x74, 0x0a, 0x0a, 0x55, 0x72, 0x6c, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12,
	0x38, 0x0a, 0x0b, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x73, 0x65, 0x55, 0x72, 0x6c, 0x73, 0x12, 0x13,
	0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x73, 0x65, 0x55, 0x72, 0x6c, 0x73, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x73, 0x65, 0x55, 0x72, 0x6c,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2c, 0x0a, 0x07, 0x47, 0x65, 0x74,
	0x55, 0x72, 0x6c, 0x73, 0x12, 0x0f, 0x2e, 0x47, 0x65, 0x74, 0x55, 0x72, 0x6c, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x10, 0x2e, 0x47, 0x65, 0x74, 0x55, 0x72, 0x6c, 0x73, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x1d, 0x5a, 0x1b, 0x73, 0x69, 0x67, 0x6d, 0x61,
	0x6f, 0x73, 0x2f, 0x73, 0x6f, 0x63, 0x69, 0x61, 0x6c, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_socialnetwork_proto_text_proto_rawDescOnce sync.Once
	file_socialnetwork_proto_text_proto_rawDescData = file_socialnetwork_proto_text_proto_rawDesc
)

func file_socialnetwork_proto_text_proto_rawDescGZIP() []byte {
	file_socialnetwork_proto_text_proto_rawDescOnce.Do(func() {
		file_socialnetwork_proto_text_proto_rawDescData = protoimpl.X.CompressGZIP(file_socialnetwork_proto_text_proto_rawDescData)
	})
	return file_socialnetwork_proto_text_proto_rawDescData
}

var file_socialnetwork_proto_text_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_socialnetwork_proto_text_proto_goTypes = []interface{}{
	(*ComposeUrlsRequest)(nil),  // 0: ComposeUrlsRequest
	(*ComposeUrlsResponse)(nil), // 1: ComposeUrlsResponse
	(*GetUrlsRequest)(nil),      // 2: GetUrlsRequest
	(*GetUrlsResponse)(nil),     // 3: GetUrlsResponse
	(*ProcessTextRequest)(nil),  // 4: ProcessTextRequest
	(*ProcessTextResponse)(nil), // 5: ProcessTextResponse
	(*UserRef)(nil),             // 6: UserRef
	(*MediaRef)(nil),            // 7: MediaRef
	(*Url)(nil),                 // 8: Url
}
var file_socialnetwork_proto_text_proto_depIdxs = []int32{
	8, // 0: ComposeUrlsResponse.urls:type_name -> Url
	6, // 1: ProcessTextResponse.usermentions:type_name -> UserRef
	8, // 2: ProcessTextResponse.urls:type_name -> Url
	4, // 3: TextService.ProcessText:input_type -> ProcessTextRequest
	0, // 4: UrlService.ComposeUrls:input_type -> ComposeUrlsRequest
	2, // 5: UrlService.GetUrls:input_type -> GetUrlsRequest
	5, // 6: TextService.ProcessText:output_type -> ProcessTextResponse
	1, // 7: UrlService.ComposeUrls:output_type -> ComposeUrlsResponse
	3, // 8: UrlService.GetUrls:output_type -> GetUrlsResponse
	6, // [6:9] is the sub-list for method output_type
	3, // [3:6] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_socialnetwork_proto_text_proto_init() }
func file_socialnetwork_proto_text_proto_init() {
	if File_socialnetwork_proto_text_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_socialnetwork_proto_text_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ComposeUrlsRequest); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ComposeUrlsResponse); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetUrlsRequest); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetUrlsResponse); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProcessTextRequest); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProcessTextResponse); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserRef); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MediaRef); i {
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
		file_socialnetwork_proto_text_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Url); i {
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
			RawDescriptor: file_socialnetwork_proto_text_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_socialnetwork_proto_text_proto_goTypes,
		DependencyIndexes: file_socialnetwork_proto_text_proto_depIdxs,
		MessageInfos:      file_socialnetwork_proto_text_proto_msgTypes,
	}.Build()
	File_socialnetwork_proto_text_proto = out.File
	file_socialnetwork_proto_text_proto_rawDesc = nil
	file_socialnetwork_proto_text_proto_goTypes = nil
	file_socialnetwork_proto_text_proto_depIdxs = nil
}
