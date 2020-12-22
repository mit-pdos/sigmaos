package fsrpc

type Fd struct {
	Addr string
}

type OpenReq struct {
	Name string
}

type OpenReply struct {
	Fd Fd
}

type MountReq struct {
	Fd   Fd
	Name string
}

type MountReply struct {
	Err error
}

type WriteReq struct {
	Buf []byte
}

type WriteReply struct {
	N int
}

type ReadReq struct {
	N int
}

type ReadReply struct {
	Buf []byte
	N   int
}
