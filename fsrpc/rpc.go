package fsrpc

import (
	"ulambda/fid"
)

type WalkReq struct {
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

type CreateReply struct {
	Fid fid.Fid
}

type OpenReq struct {
	Fid fid.Fid
}

type OpenReply struct {
	Err error
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

type PipeReq struct {
	Fid  fid.Fid
	Name string
}

type PipeReply struct {
	Err error
}

type MountReq struct {
	Ufid fid.Ufid
	Fid  fid.Fid
	Name string
}

type MountReply struct {
	Err error
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
