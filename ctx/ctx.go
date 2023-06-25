package ctx

import (
	"sigmaos/sesscond"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Ctx struct {
	uname  sp.Tuname
	sessid sessp.Tsession
	clntid sp.TclntId
	sct    *sesscond.SessCondTable
}

func MkCtx(uname sp.Tuname, sessid sessp.Tsession, clntid sp.TclntId, sct *sesscond.SessCondTable) *Ctx {
	return &Ctx{uname, sessid, clntid, sct}
}

func MkCtxNull() *Ctx {
	return MkCtx("", 0, sp.NoClntId, nil)
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
