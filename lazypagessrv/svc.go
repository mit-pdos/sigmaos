package lazypagessrv

import (
	db "sigmaos/debug"

	"sigmaos/fs"
	"sigmaos/lazypagessrv/proto"
)

//
// RPC interface
//

type LazyPagesSvc struct {
	lps *lazyPagesSrv
}

func (lps *LazyPagesSvc) Register(ctx fs.CtxI, req proto.RegisterRequest, res *proto.RegisterResult) error {
	db.DPrintf(db.LAZYPAGESSRV, "Register %v", req)
	return lps.lps.register(int(req.Pid), req.ImgDir, req.Pages)
}
