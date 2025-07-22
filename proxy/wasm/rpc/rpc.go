package rpc

type RPCAPI interface {
	Send(rpcIdx uint64, pn string, b []byte) error
	Recv(rpcIdx uint64) ([]byte, error)
}
