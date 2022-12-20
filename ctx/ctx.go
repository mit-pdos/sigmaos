package ctx

import (
	"sigmaos/sessp"
	"sigmaos/sesscond"
)

type Ctx struct {
	uname  string
	sessid sessp.Tsession
	sct    *sesscond.SessCondTable
}

func MkCtx(uname string, sessid sessp.Tsession, sct *sesscond.SessCondTable) *Ctx {
	return &Ctx{uname, sessid, sct}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}

func (ctx *Ctx) SessionId() sessp.Tsession {
	return ctx.sessid
}

func (ctx *Ctx) SessCondTable() *sesscond.SessCondTable {
	return ctx.sct
}
