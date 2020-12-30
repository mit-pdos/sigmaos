package fsrpc

import (
	"ulambda/fid"
)

type NameReq struct {
	Fid  fid.Fid
	Name string
}

type WalkReply struct {
	Path string
	Ufid fid.Ufid
}

type CreateReq struct {
	Fid  fid.Fid
	Name string
	Type fid.IType
}

type FidReq struct {
	Fid fid.Fid
}

type FidReply struct {
	Fid fid.Fid
}

type EmptyReply struct {
}

type SymlinkReq struct {
	Fid   fid.Fid
	Src   string
	Start fid.Ufid
	Dst   string
}

type SymlinkReply struct {
	Err error
}

type MountReq struct {
	Ufid fid.Ufid
	Fid  fid.Fid
	Name string
}

type WriteReq struct {
	Fid fid.Fid
	Buf []byte
}

type WriteReply struct {
	N int
}

type ReadReq struct {
	Fid fid.Fid
	N   int
}

type ReadReply struct {
	Buf []byte
	N   int
}
