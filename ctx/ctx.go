package ctx

import (
	"sigmaos/clntcond"
	"sigmaos/fs"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Ctx struct {
	uname   sp.Tuname
	sessid  sessp.Tsession
	clntid  sp.TclntId
	sct     *clntcond.ClntCondTable
	fencefs fs.Dir
}

func NewCtx(uname sp.Tuname, sessid sessp.Tsession, clntid sp.TclntId, sct *clntcond.ClntCondTable, fencefs fs.Dir) *Ctx {
	return &Ctx{uname: uname, sessid: sessid, clntid: clntid, sct: sct, fencefs: fencefs}
}

func NewCtxNull() *Ctx {
	return NewCtx("", 0, sp.NoClntId, nil, nil)
}

func (ctx *Ctx) Uname() sp.Tuname {
	return ctx.uname
}

func (ctx *Ctx) SessionId() sessp.Tsession {
	return ctx.sessid
}

func (ctx *Ctx) ClntId() sp.TclntId {
	return ctx.clntid
}

func (ctx *Ctx) ClntCondTable() *clntcond.ClntCondTable {
	return ctx.sct
}

func (ctx *Ctx) FenceFs() fs.Dir {
	return ctx.fencefs
}
