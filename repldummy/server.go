package repldummy

import (
	"sigmaos/sessp"
	"sigmaos/threadmgr"
)

type DummyReplServer struct {
	tm *threadmgr.ThreadMgr
}

func MakeDummyReplServer(tm *threadmgr.ThreadMgr) *DummyReplServer {
	srv := &DummyReplServer{}
	srv.tm = tm
	return srv
}

func (srv *DummyReplServer) Start() {
}

func (srv *DummyReplServer) Process(fc *sessp.FcallMsg) {
	srv.tm.Process(fc)
}
