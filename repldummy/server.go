package repldummy

import (
	np "ulambda/ninep"
	"ulambda/threadmgr"
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

func (srv *DummyReplServer) Process(fc *np.Fcall, replies chan *np.Fcall) {
	srv.tm.Process(fc, replies)
}
