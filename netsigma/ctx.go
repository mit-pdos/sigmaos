package netsigma

import (
	"os"

	"sigmaos/fs"
)

type WrapperCtx struct {
	fs.CtxI
	file *os.File
	ok   bool
}

// An fs.CtxI wrapper, used to pass the proxied FD back past the rpcsrv layer
func NewWrapperCtx(ctx fs.CtxI) *WrapperCtx {
	return &WrapperCtx{
		CtxI: ctx,
	}
}

func (ctx *WrapperCtx) SetFile(f *os.File) {
	ctx.file = f
	ctx.ok = true
}

func (ctx *WrapperCtx) GetFile() (*os.File, bool) {
	return ctx.file, ctx.ok
}
