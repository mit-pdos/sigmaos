package rpc

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	rpcproto "sigmaos/rpc/proto"
)

// return the Blob message, if this message contains one
func GetBlob(msg proto.Message) *rpcproto.Blob {
	var blob *rpcproto.Blob
	msg.ProtoReflect().Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Kind() == protoreflect.MessageKind {
			if m := f.Message(); m.FullName() == "Blob" {
				blob = v.Message().Interface().(*rpcproto.Blob)
				return false
			}
		}
		return true
	})
	return blob
}
