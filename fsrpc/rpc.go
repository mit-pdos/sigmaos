package fsrpc

const (
	NullId uint64 = 0
	RootId uint64 = 1
)

type Fid struct {
	Version uint64
	Id      uint64
}

func MakeFid(v uint64, id uint64) Fid {
	return Fid{v, id}
}

func NullFid() Fid {
	return MakeFid(0, NullId)
}

func RootFid() Fid {
	return MakeFid(0, RootId)
}

type Ufid struct {
	Addr string
	Fid  Fid
}

func MakeUfid(addr string, fid Fid) *Ufid {
	return &Ufid{addr, fid}
}

type WalkReq struct {
	Fid  Fid
	Name string
}

type WalkReply struct {
	Path string
	Ufid Ufid
}

type CreateReq struct {
	Fid  Fid
	Name string
}

type CreateReply struct {
	Fid Fid
}

type OpenReq struct {
	Fid  Fid
	Name string
}

type OpenReply struct {
	Fid Fid
}

type MountReq struct {
	Ufid Ufid
	Fid  Fid
	Name string
}

type MountReply struct {
	Err error
}

type WriteReq struct {
	Fid Fid
	Buf []byte
}

type WriteReply struct {
	N int
}

type ReadReq struct {
	Fid Fid
	N   int
}

type ReadReply struct {
	Buf []byte
	N   int
}
