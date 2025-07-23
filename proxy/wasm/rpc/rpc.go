package rpc

type RPCAPI interface {
	Send(rpcIdx uint64, pn string, method string, b []byte, nOutIOV uint64) error
	Recv(rpcIdx uint64) ([]byte, error)
}
