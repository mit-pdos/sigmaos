// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.25.1
// source: sigmaclntsrv/proto/sigmaclntsrv.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sigmap "sigmaos/sigmap"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SigmaCloseRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fd uint32 `protobuf:"varint,1,opt,name=fd,proto3" json:"fd,omitempty"`
}

func (x *SigmaCloseRequest) Reset() {
	*x = SigmaCloseRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaCloseRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaCloseRequest) ProtoMessage() {}

func (x *SigmaCloseRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaCloseRequest.ProtoReflect.Descriptor instead.
func (*SigmaCloseRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{0}
}

func (x *SigmaCloseRequest) GetFd() uint32 {
	if x != nil {
		return x.Fd
	}
	return 0
}

type SigmaErrReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Err *sigmap.Rerror `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
}

func (x *SigmaErrReply) Reset() {
	*x = SigmaErrReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaErrReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaErrReply) ProtoMessage() {}

func (x *SigmaErrReply) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaErrReply.ProtoReflect.Descriptor instead.
func (*SigmaErrReply) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{1}
}

func (x *SigmaErrReply) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

type SigmaStatRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *SigmaStatRequest) Reset() {
	*x = SigmaStatRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaStatRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaStatRequest) ProtoMessage() {}

func (x *SigmaStatRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaStatRequest.ProtoReflect.Descriptor instead.
func (*SigmaStatRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{2}
}

func (x *SigmaStatRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

type SigmaStatReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Stat *sigmap.Stat   `protobuf:"bytes,1,opt,name=stat,proto3" json:"stat,omitempty"`
	Err  *sigmap.Rerror `protobuf:"bytes,2,opt,name=err,proto3" json:"err,omitempty"`
}

func (x *SigmaStatReply) Reset() {
	*x = SigmaStatReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaStatReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaStatReply) ProtoMessage() {}

func (x *SigmaStatReply) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaStatReply.ProtoReflect.Descriptor instead.
func (*SigmaStatReply) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{3}
}

func (x *SigmaStatReply) GetStat() *sigmap.Stat {
	if x != nil {
		return x.Stat
	}
	return nil
}

func (x *SigmaStatReply) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

type SigmaCreateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Perm uint32 `protobuf:"varint,2,opt,name=perm,proto3" json:"perm,omitempty"`
	Mode uint32 `protobuf:"varint,3,opt,name=mode,proto3" json:"mode,omitempty"`
}

func (x *SigmaCreateRequest) Reset() {
	*x = SigmaCreateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaCreateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaCreateRequest) ProtoMessage() {}

func (x *SigmaCreateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaCreateRequest.ProtoReflect.Descriptor instead.
func (*SigmaCreateRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{4}
}

func (x *SigmaCreateRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *SigmaCreateRequest) GetPerm() uint32 {
	if x != nil {
		return x.Perm
	}
	return 0
}

func (x *SigmaCreateRequest) GetMode() uint32 {
	if x != nil {
		return x.Mode
	}
	return 0
}

type SigmaFdReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fd  uint32         `protobuf:"varint,1,opt,name=fd,proto3" json:"fd,omitempty"`
	Err *sigmap.Rerror `protobuf:"bytes,2,opt,name=err,proto3" json:"err,omitempty"`
}

func (x *SigmaFdReply) Reset() {
	*x = SigmaFdReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaFdReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaFdReply) ProtoMessage() {}

func (x *SigmaFdReply) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaFdReply.ProtoReflect.Descriptor instead.
func (*SigmaFdReply) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{5}
}

func (x *SigmaFdReply) GetFd() uint32 {
	if x != nil {
		return x.Fd
	}
	return 0
}

func (x *SigmaFdReply) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

type SigmaRenameRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Src string `protobuf:"bytes,1,opt,name=src,proto3" json:"src,omitempty"`
	Dst string `protobuf:"bytes,2,opt,name=dst,proto3" json:"dst,omitempty"`
}

func (x *SigmaRenameRequest) Reset() {
	*x = SigmaRenameRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaRenameRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaRenameRequest) ProtoMessage() {}

func (x *SigmaRenameRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaRenameRequest.ProtoReflect.Descriptor instead.
func (*SigmaRenameRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{6}
}

func (x *SigmaRenameRequest) GetSrc() string {
	if x != nil {
		return x.Src
	}
	return ""
}

func (x *SigmaRenameRequest) GetDst() string {
	if x != nil {
		return x.Dst
	}
	return ""
}

type SigmaRemoveRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *SigmaRemoveRequest) Reset() {
	*x = SigmaRemoveRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaRemoveRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaRemoveRequest) ProtoMessage() {}

func (x *SigmaRemoveRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaRemoveRequest.ProtoReflect.Descriptor instead.
func (*SigmaRemoveRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{7}
}

func (x *SigmaRemoveRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

type SigmaGetFileRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *SigmaGetFileRequest) Reset() {
	*x = SigmaGetFileRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaGetFileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaGetFileRequest) ProtoMessage() {}

func (x *SigmaGetFileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaGetFileRequest.ProtoReflect.Descriptor instead.
func (*SigmaGetFileRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{8}
}

func (x *SigmaGetFileRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

type SigmaDataReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte         `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Err  *sigmap.Rerror `protobuf:"bytes,2,opt,name=err,proto3" json:"err,omitempty"`
}

func (x *SigmaDataReply) Reset() {
	*x = SigmaDataReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaDataReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaDataReply) ProtoMessage() {}

func (x *SigmaDataReply) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaDataReply.ProtoReflect.Descriptor instead.
func (*SigmaDataReply) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{9}
}

func (x *SigmaDataReply) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *SigmaDataReply) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

type SigmaPutFileRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path    string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Perm    uint32 `protobuf:"varint,2,opt,name=perm,proto3" json:"perm,omitempty"`
	Mode    uint32 `protobuf:"varint,3,opt,name=mode,proto3" json:"mode,omitempty"`
	Offset  uint64 `protobuf:"varint,4,opt,name=offset,proto3" json:"offset,omitempty"`
	LeaseId uint64 `protobuf:"varint,5,opt,name=leaseId,proto3" json:"leaseId,omitempty"`
	Data    []byte `protobuf:"bytes,6,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *SigmaPutFileRequest) Reset() {
	*x = SigmaPutFileRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaPutFileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaPutFileRequest) ProtoMessage() {}

func (x *SigmaPutFileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaPutFileRequest.ProtoReflect.Descriptor instead.
func (*SigmaPutFileRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{10}
}

func (x *SigmaPutFileRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *SigmaPutFileRequest) GetPerm() uint32 {
	if x != nil {
		return x.Perm
	}
	return 0
}

func (x *SigmaPutFileRequest) GetMode() uint32 {
	if x != nil {
		return x.Mode
	}
	return 0
}

func (x *SigmaPutFileRequest) GetOffset() uint64 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *SigmaPutFileRequest) GetLeaseId() uint64 {
	if x != nil {
		return x.LeaseId
	}
	return 0
}

func (x *SigmaPutFileRequest) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type SigmaSizeReply struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Size uint64         `protobuf:"varint,1,opt,name=size,proto3" json:"size,omitempty"`
	Err  *sigmap.Rerror `protobuf:"bytes,2,opt,name=err,proto3" json:"err,omitempty"`
}

func (x *SigmaSizeReply) Reset() {
	*x = SigmaSizeReply{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaSizeReply) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaSizeReply) ProtoMessage() {}

func (x *SigmaSizeReply) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaSizeReply.ProtoReflect.Descriptor instead.
func (*SigmaSizeReply) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{11}
}

func (x *SigmaSizeReply) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *SigmaSizeReply) GetErr() *sigmap.Rerror {
	if x != nil {
		return x.Err
	}
	return nil
}

type SigmaReadRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fd   uint32 `protobuf:"varint,1,opt,name=fd,proto3" json:"fd,omitempty"`
	Size uint64 `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
}

func (x *SigmaReadRequest) Reset() {
	*x = SigmaReadRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[12]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaReadRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaReadRequest) ProtoMessage() {}

func (x *SigmaReadRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[12]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaReadRequest.ProtoReflect.Descriptor instead.
func (*SigmaReadRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{12}
}

func (x *SigmaReadRequest) GetFd() uint32 {
	if x != nil {
		return x.Fd
	}
	return 0
}

func (x *SigmaReadRequest) GetSize() uint64 {
	if x != nil {
		return x.Size
	}
	return 0
}

type SigmaWriteRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fd   uint32 `protobuf:"varint,1,opt,name=fd,proto3" json:"fd,omitempty"`
	Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *SigmaWriteRequest) Reset() {
	*x = SigmaWriteRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[13]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaWriteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaWriteRequest) ProtoMessage() {}

func (x *SigmaWriteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[13]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaWriteRequest.ProtoReflect.Descriptor instead.
func (*SigmaWriteRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{13}
}

func (x *SigmaWriteRequest) GetFd() uint32 {
	if x != nil {
		return x.Fd
	}
	return 0
}

func (x *SigmaWriteRequest) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type SigmaSeekRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fd     uint32 `protobuf:"varint,1,opt,name=fd,proto3" json:"fd,omitempty"`
	Offset uint64 `protobuf:"varint,2,opt,name=offset,proto3" json:"offset,omitempty"`
}

func (x *SigmaSeekRequest) Reset() {
	*x = SigmaSeekRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[14]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SigmaSeekRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SigmaSeekRequest) ProtoMessage() {}

func (x *SigmaSeekRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[14]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SigmaSeekRequest.ProtoReflect.Descriptor instead.
func (*SigmaSeekRequest) Descriptor() ([]byte, []int) {
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP(), []int{14}
}

func (x *SigmaSeekRequest) GetFd() uint32 {
	if x != nil {
		return x.Fd
	}
	return 0
}

func (x *SigmaSeekRequest) GetOffset() uint64 {
	if x != nil {
		return x.Offset
	}
	return 0
}

var File_sigmaclntsrv_proto_sigmaclntsrv_proto protoreflect.FileDescriptor

var file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDesc = []byte{
	0x0a, 0x25, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x63, 0x6c, 0x6e, 0x74, 0x73, 0x72, 0x76, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x63, 0x6c, 0x6e, 0x74, 0x73, 0x72,
	0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x13, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2f,
	0x73, 0x69, 0x67, 0x6d, 0x61, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x23, 0x0a, 0x11,
	0x53, 0x69, 0x67, 0x6d, 0x61, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x0e, 0x0a, 0x02, 0x66, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x66,
	0x64, 0x22, 0x2a, 0x0a, 0x0d, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x45, 0x72, 0x72, 0x52, 0x65, 0x70,
	0x6c, 0x79, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x22, 0x26, 0x0a,
	0x10, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x53, 0x74, 0x61, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x70, 0x61, 0x74, 0x68, 0x22, 0x46, 0x0a, 0x0e, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x53, 0x74,
	0x61, 0x74, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x19, 0x0a, 0x04, 0x73, 0x74, 0x61, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x52, 0x04, 0x73, 0x74,
	0x61, 0x74, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x22, 0x50, 0x0a,
	0x12, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x6d,
	0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x22,
	0x39, 0x0a, 0x0c, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x46, 0x64, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12,
	0x0e, 0x0a, 0x02, 0x66, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x66, 0x64, 0x12,
	0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x52,
	0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x22, 0x38, 0x0a, 0x12, 0x53, 0x69,
	0x67, 0x6d, 0x61, 0x52, 0x65, 0x6e, 0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x10, 0x0a, 0x03, 0x73, 0x72, 0x63, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x73,
	0x72, 0x63, 0x12, 0x10, 0x0a, 0x03, 0x64, 0x73, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x64, 0x73, 0x74, 0x22, 0x28, 0x0a, 0x12, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x52, 0x65, 0x6d,
	0x6f, 0x76, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61,
	0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x22, 0x29,
	0x0a, 0x13, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x47, 0x65, 0x74, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x22, 0x3f, 0x0a, 0x0e, 0x53, 0x69, 0x67,
	0x6d, 0x61, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12,
	0x19, 0x0a, 0x03, 0x65, 0x72, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x52,
	0x65, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x03, 0x65, 0x72, 0x72, 0x22, 0x97, 0x01, 0x0a, 0x13, 0x53,
	0x69, 0x67, 0x6d, 0x61, 0x50, 0x75, 0x74, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f,
	0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06,
	0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x49,
	0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x49, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x22, 0x3f, 0x0a, 0x0e, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x53, 0x69, 0x7a,
	0x65, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x19, 0x0a, 0x03, 0x65, 0x72,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x52, 0x65, 0x72, 0x72, 0x6f, 0x72,
	0x52, 0x03, 0x65, 0x72, 0x72, 0x22, 0x36, 0x0a, 0x10, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x52, 0x65,
	0x61, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x66, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x66, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x22, 0x37, 0x0a,
	0x11, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x57, 0x72, 0x69, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x66, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02,
	0x66, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x3a, 0x0a, 0x10, 0x53, 0x69, 0x67, 0x6d, 0x61, 0x53,
	0x65, 0x65, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x66, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x66, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66,
	0x66, 0x73, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73,
	0x65, 0x74, 0x42, 0x1c, 0x5a, 0x1a, 0x73, 0x69, 0x67, 0x6d, 0x61, 0x6f, 0x73, 0x2f, 0x73, 0x69,
	0x67, 0x6d, 0x61, 0x63, 0x6c, 0x6e, 0x74, 0x73, 0x72, 0x76, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescOnce sync.Once
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescData = file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDesc
)

func file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescGZIP() []byte {
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescOnce.Do(func() {
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescData = protoimpl.X.CompressGZIP(file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescData)
	})
	return file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDescData
}

var file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes = make([]protoimpl.MessageInfo, 15)
var file_sigmaclntsrv_proto_sigmaclntsrv_proto_goTypes = []interface{}{
	(*SigmaCloseRequest)(nil),   // 0: SigmaCloseRequest
	(*SigmaErrReply)(nil),       // 1: SigmaErrReply
	(*SigmaStatRequest)(nil),    // 2: SigmaStatRequest
	(*SigmaStatReply)(nil),      // 3: SigmaStatReply
	(*SigmaCreateRequest)(nil),  // 4: SigmaCreateRequest
	(*SigmaFdReply)(nil),        // 5: SigmaFdReply
	(*SigmaRenameRequest)(nil),  // 6: SigmaRenameRequest
	(*SigmaRemoveRequest)(nil),  // 7: SigmaRemoveRequest
	(*SigmaGetFileRequest)(nil), // 8: SigmaGetFileRequest
	(*SigmaDataReply)(nil),      // 9: SigmaDataReply
	(*SigmaPutFileRequest)(nil), // 10: SigmaPutFileRequest
	(*SigmaSizeReply)(nil),      // 11: SigmaSizeReply
	(*SigmaReadRequest)(nil),    // 12: SigmaReadRequest
	(*SigmaWriteRequest)(nil),   // 13: SigmaWriteRequest
	(*SigmaSeekRequest)(nil),    // 14: SigmaSeekRequest
	(*sigmap.Rerror)(nil),       // 15: Rerror
	(*sigmap.Stat)(nil),         // 16: Stat
}
var file_sigmaclntsrv_proto_sigmaclntsrv_proto_depIdxs = []int32{
	15, // 0: SigmaErrReply.err:type_name -> Rerror
	16, // 1: SigmaStatReply.stat:type_name -> Stat
	15, // 2: SigmaStatReply.err:type_name -> Rerror
	15, // 3: SigmaFdReply.err:type_name -> Rerror
	15, // 4: SigmaDataReply.err:type_name -> Rerror
	15, // 5: SigmaSizeReply.err:type_name -> Rerror
	6,  // [6:6] is the sub-list for method output_type
	6,  // [6:6] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_sigmaclntsrv_proto_sigmaclntsrv_proto_init() }
func file_sigmaclntsrv_proto_sigmaclntsrv_proto_init() {
	if File_sigmaclntsrv_proto_sigmaclntsrv_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaCloseRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaErrReply); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaStatRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaStatReply); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaCreateRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaFdReply); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaRenameRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaRemoveRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaGetFileRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaDataReply); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaPutFileRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaSizeReply); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[12].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaReadRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[13].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaWriteRequest); i {
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
		file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes[14].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SigmaSeekRequest); i {
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
			RawDescriptor: file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   15,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_sigmaclntsrv_proto_sigmaclntsrv_proto_goTypes,
		DependencyIndexes: file_sigmaclntsrv_proto_sigmaclntsrv_proto_depIdxs,
		MessageInfos:      file_sigmaclntsrv_proto_sigmaclntsrv_proto_msgTypes,
	}.Build()
	File_sigmaclntsrv_proto_sigmaclntsrv_proto = out.File
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_rawDesc = nil
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_goTypes = nil
	file_sigmaclntsrv_proto_sigmaclntsrv_proto_depIdxs = nil
}