// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: apps/socialnetwork/proto/user.proto

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

type CheckUserReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Usernames []string `protobuf:"bytes,1,rep,name=usernames,proto3" json:"usernames,omitempty"`
}

func (x *CheckUserReq) Reset() {
	*x = CheckUserReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CheckUserReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckUserReq) ProtoMessage() {}

func (x *CheckUserReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckUserReq.ProtoReflect.Descriptor instead.
func (*CheckUserReq) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{0}
}

func (x *CheckUserReq) GetUsernames() []string {
	if x != nil {
		return x.Usernames
	}
	return nil
}

type CheckUserRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok      string  `protobuf:"bytes,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Userids []int64 `protobuf:"varint,2,rep,packed,name=userids,proto3" json:"userids,omitempty"`
}

func (x *CheckUserRep) Reset() {
	*x = CheckUserRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CheckUserRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CheckUserRep) ProtoMessage() {}

func (x *CheckUserRep) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CheckUserRep.ProtoReflect.Descriptor instead.
func (*CheckUserRep) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{1}
}

func (x *CheckUserRep) GetOk() string {
	if x != nil {
		return x.Ok
	}
	return ""
}

func (x *CheckUserRep) GetUserids() []int64 {
	if x != nil {
		return x.Userids
	}
	return nil
}

type RegisterUserReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Username  string `protobuf:"bytes,1,opt,name=username,proto3" json:"username,omitempty"`
	Password  string `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	Firstname string `protobuf:"bytes,3,opt,name=firstname,proto3" json:"firstname,omitempty"`
	Lastname  string `protobuf:"bytes,4,opt,name=lastname,proto3" json:"lastname,omitempty"`
}

func (x *RegisterUserReq) Reset() {
	*x = RegisterUserReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RegisterUserReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegisterUserReq) ProtoMessage() {}

func (x *RegisterUserReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegisterUserReq.ProtoReflect.Descriptor instead.
func (*RegisterUserReq) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{2}
}

func (x *RegisterUserReq) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *RegisterUserReq) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

func (x *RegisterUserReq) GetFirstname() string {
	if x != nil {
		return x.Firstname
	}
	return ""
}

func (x *RegisterUserReq) GetLastname() string {
	if x != nil {
		return x.Lastname
	}
	return ""
}

type LoginReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Username string `protobuf:"bytes,1,opt,name=username,proto3" json:"username,omitempty"`
	Password string `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
}

func (x *LoginReq) Reset() {
	*x = LoginReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LoginReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LoginReq) ProtoMessage() {}

func (x *LoginReq) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LoginReq.ProtoReflect.Descriptor instead.
func (*LoginReq) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{3}
}

func (x *LoginReq) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *LoginReq) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

type UserRep struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ok     string `protobuf:"bytes,1,opt,name=ok,proto3" json:"ok,omitempty"`
	Userid int64  `protobuf:"varint,2,opt,name=userid,proto3" json:"userid,omitempty"`
}

func (x *UserRep) Reset() {
	*x = UserRep{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserRep) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserRep) ProtoMessage() {}

func (x *UserRep) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserRep.ProtoReflect.Descriptor instead.
func (*UserRep) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{4}
}

func (x *UserRep) GetOk() string {
	if x != nil {
		return x.Ok
	}
	return ""
}

func (x *UserRep) GetUserid() int64 {
	if x != nil {
		return x.Userid
	}
	return 0
}

type CacheItem struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Val []byte `protobuf:"bytes,2,opt,name=val,proto3" json:"val,omitempty"`
}

func (x *CacheItem) Reset() {
	*x = CacheItem{}
	if protoimpl.UnsafeEnabled {
		mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CacheItem) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CacheItem) ProtoMessage() {}

func (x *CacheItem) ProtoReflect() protoreflect.Message {
	mi := &file_apps_socialnetwork_proto_user_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CacheItem.ProtoReflect.Descriptor instead.
func (*CacheItem) Descriptor() ([]byte, []int) {
	return file_apps_socialnetwork_proto_user_proto_rawDescGZIP(), []int{5}
}

func (x *CacheItem) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *CacheItem) GetVal() []byte {
	if x != nil {
		return x.Val
	}
	return nil
}

var File_apps_socialnetwork_proto_user_proto protoreflect.FileDescriptor

var file_apps_socialnetwork_proto_user_proto_rawDesc = []byte{
	0x0a, 0x23, 0x61, 0x70, 0x70, 0x73, 0x2f, 0x73, 0x6f, 0x63, 0x69, 0x61, 0x6c, 0x6e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x75, 0x73, 0x65, 0x72, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2c, 0x0a, 0x0c, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x55, 0x73,
	0x65, 0x72, 0x52, 0x65, 0x71, 0x12, 0x1c, 0x0a, 0x09, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d,
	0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x09, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x22, 0x38, 0x0a, 0x0c, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x55, 0x73, 0x65, 0x72,
	0x52, 0x65, 0x70, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x6f, 0x6b, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x73, 0x65, 0x72, 0x69, 0x64, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x03, 0x52, 0x07, 0x75, 0x73, 0x65, 0x72, 0x69, 0x64, 0x73, 0x22, 0x83, 0x01,
	0x0a, 0x0f, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65, 0x72, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65,
	0x71, 0x12, 0x1a, 0x0a, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a,
	0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x66, 0x69, 0x72,
	0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x66, 0x69,
	0x72, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x61, 0x73, 0x74, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6c, 0x61, 0x73, 0x74, 0x6e,
	0x61, 0x6d, 0x65, 0x22, 0x42, 0x0a, 0x08, 0x4c, 0x6f, 0x67, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x12,
	0x1a, 0x0a, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x70,
	0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70,
	0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x22, 0x31, 0x0a, 0x07, 0x55, 0x73, 0x65, 0x72, 0x52,
	0x65, 0x70, 0x12, 0x0e, 0x0a, 0x02, 0x6f, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x6f, 0x6b, 0x12, 0x16, 0x0a, 0x06, 0x75, 0x73, 0x65, 0x72, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x69, 0x64, 0x22, 0x2f, 0x0a, 0x09, 0x43, 0x61,
	0x63, 0x68, 0x65, 0x49, 0x74, 0x65, 0x6d, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x76, 0x61, 0x6c,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x76, 0x61, 0x6c, 0x32, 0x82, 0x01, 0x0a, 0x0b,
	0x55, 0x73, 0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x29, 0x0a, 0x09, 0x43,
	0x68, 0x65, 0x63, 0x6b, 0x55, 0x73, 0x65, 0x72, 0x12, 0x0d, 0x2e, 0x43, 0x68, 0x65, 0x63, 0x6b,
	0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x71, 0x1a, 0x0d, 0x2e, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x55,
	0x73, 0x65, 0x72, 0x52, 0x65, 0x70, 0x12, 0x2a, 0x0a, 0x0c, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74,
	0x65, 0x72, 0x55, 0x73, 0x65, 0x72, 0x12, 0x10, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x65,
	0x72, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x71, 0x1a, 0x08, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52,
	0x65, 0x70, 0x12, 0x1c, 0x0a, 0x05, 0x4c, 0x6f, 0x67, 0x69, 0x6e, 0x12, 0x09, 0x2e, 0x4c, 0x6f,
	0x67, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x1a, 0x08, 0x2e, 0x55, 0x73, 0x65, 0x72, 0x52, 0x65, 0x70,
	0x42, 0x22, 0x5a, 0x20, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x61, 0x70, 0x70, 0x73,
	0x2f, 0x73, 0x6f, 0x63, 0x69, 0x61, 0x6c, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_apps_socialnetwork_proto_user_proto_rawDescOnce sync.Once
	file_apps_socialnetwork_proto_user_proto_rawDescData = file_apps_socialnetwork_proto_user_proto_rawDesc
)

func file_apps_socialnetwork_proto_user_proto_rawDescGZIP() []byte {
	file_apps_socialnetwork_proto_user_proto_rawDescOnce.Do(func() {
		file_apps_socialnetwork_proto_user_proto_rawDescData = protoimpl.X.CompressGZIP(file_apps_socialnetwork_proto_user_proto_rawDescData)
	})
	return file_apps_socialnetwork_proto_user_proto_rawDescData
}

var file_apps_socialnetwork_proto_user_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_apps_socialnetwork_proto_user_proto_goTypes = []interface{}{
	(*CheckUserReq)(nil),    // 0: CheckUserReq
	(*CheckUserRep)(nil),    // 1: CheckUserRep
	(*RegisterUserReq)(nil), // 2: RegisterUserReq
	(*LoginReq)(nil),        // 3: LoginReq
	(*UserRep)(nil),         // 4: UserRep
	(*CacheItem)(nil),       // 5: CacheItem
}
var file_apps_socialnetwork_proto_user_proto_depIdxs = []int32{
	0, // 0: UserService.CheckUser:input_type -> CheckUserReq
	2, // 1: UserService.RegisterUser:input_type -> RegisterUserReq
	3, // 2: UserService.Login:input_type -> LoginReq
	1, // 3: UserService.CheckUser:output_type -> CheckUserRep
	4, // 4: UserService.RegisterUser:output_type -> UserRep
	4, // 5: UserService.Login:output_type -> UserRep
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_apps_socialnetwork_proto_user_proto_init() }
func file_apps_socialnetwork_proto_user_proto_init() {
	if File_apps_socialnetwork_proto_user_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_apps_socialnetwork_proto_user_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CheckUserReq); i {
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
		file_apps_socialnetwork_proto_user_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CheckUserRep); i {
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
		file_apps_socialnetwork_proto_user_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RegisterUserReq); i {
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
		file_apps_socialnetwork_proto_user_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LoginReq); i {
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
		file_apps_socialnetwork_proto_user_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserRep); i {
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
		file_apps_socialnetwork_proto_user_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CacheItem); i {
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
			RawDescriptor: file_apps_socialnetwork_proto_user_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_apps_socialnetwork_proto_user_proto_goTypes,
		DependencyIndexes: file_apps_socialnetwork_proto_user_proto_depIdxs,
		MessageInfos:      file_apps_socialnetwork_proto_user_proto_msgTypes,
	}.Build()
	File_apps_socialnetwork_proto_user_proto = out.File
	file_apps_socialnetwork_proto_user_proto_rawDesc = nil
	file_apps_socialnetwork_proto_user_proto_goTypes = nil
	file_apps_socialnetwork_proto_user_proto_depIdxs = nil
}
