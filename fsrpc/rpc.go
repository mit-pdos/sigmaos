package fsrpc

type Fd uint64

type Ufd struct {
	Addr string
	Fd   Fd
}

func MakeUfd(addr string, fd Fd) *Ufd {
	return &Ufd{addr, fd}
}

type WalkReq struct {
	Name string
}

type WalkReply struct {
	Ufd Ufd
}

type CreateReq struct {
	Name string
}

type CreateReply struct {
	Fd Fd
}

type OpenReq struct {
	Name string
}

type OpenReply struct {
	Fd Fd
}

type MountReq struct {
	Ufd  Ufd
	Name string
}

type MountReply struct {
	Err error
}

type WriteReq struct {
	Fd  Fd
	Buf []byte
}

type WriteReply struct {
	N int
}

type ReadReq struct {
	Fd Fd
	N  int
}

type ReadReply struct {
	Buf []byte
	N   int
}
