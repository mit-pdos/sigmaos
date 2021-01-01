package fsclnt

import (
	"log"
	"net/rpc"
)

type ChanMgr struct {
	clnts map[string]*rpc.Client
}

func makeChanMgr() *ChanMgr {
	cm := &ChanMgr{}
	cm.clnts = make(map[string]*rpc.Client)
	return cm
}

func (cm *ChanMgr) makeCall(addr string, method string, req interface{},
	reply interface{}) error {
	log.Printf("makeCall %s at %v %v\n", method, addr, req)
	clnt, ok := cm.clnts[addr]
	if !ok {
		var err error
		clnt, err = rpc.Dial("tcp", addr)
		if err != nil {
			return err
		}
		cm.clnts[addr] = clnt
	}
	return clnt.Call(method, req, reply)
}
