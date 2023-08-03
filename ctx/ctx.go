package ctx

import (
	"sigmaos/fs"
	"sigmaos/sesscond"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Ctx struct {
	uname   sp.Tuname
	sessid  sessp.Tsession
	clntid  sp.TclntId
	sct     *sesscond.SessCondTable
	fencefs fs.Dir
}

func MkCtx(uname sp.Tuname, sessid sessp.Tsession, clntid sp.TclntId, sct *sesscond.SessCondTable, fencefs fs.Dir) *Ctx {
	return &Ctx{uname: uname, sessid: sessid, clntid: clntid, sct: sct, fencefs: fencefs}
}

func MkCtxNull() *Ctx {
	return MkCtx("", 0, sp.NoClntId, nil, nil)
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

func (ctx *Ctx) SessCondTable() *sesscond.SessCondTable {
	return ctx.sct
}

func (ctx *Ctx) FenceFs() fs.Dir {
	return ctx.fencefs
}
