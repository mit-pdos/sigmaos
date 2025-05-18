package rpcchannel

import (
	"net"

	sp "sigmaos/sigmap"
)

func NewTCPConnChannel(ep *sp.Tendpoint) (*RPCChannel, error) {
	conn, err := net.Dial("tcp", ep.Addrs()[0].IPPort())
	if err != nil {
		return nil, err
	}
	return NewRPCChannel(conn), nil
}

func NewUnixConnChannel(pn string) (*RPCChannel, error) {
	conn, err := net.Dial("unix", pn)
	if err != nil {
		return nil, err
	}
	return NewRPCChannel(conn), nil
}
