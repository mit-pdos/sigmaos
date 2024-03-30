package netsigma

import (
	"sigmaos/fs"
)

type WrapperCtx struct {
	fs.CtxI
	fd int
	ok bool
}

// An fs.CtxI wrapper, used to pass the proxied FD back past the rpcsrv layer
func NewWrapperCtx(ctx fs.CtxI) *WrapperCtx {
	return &WrapperCtx{
		CtxI: ctx,
		fd:   0,
	}
}

func (ctx *WrapperCtx) SetFD(fd int) {
	ctx.fd = fd
	ctx.ok = true
}

func (ctx *WrapperCtx) GetFD() (int, bool) {
	return ctx.fd, ctx.ok
}
