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
	lps.lps.imgdir = req.ImgDir
	lps.lps.pages = req.Pages
	return nil
}
