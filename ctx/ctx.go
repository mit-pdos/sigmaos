package ctx

import (
	np "sigmaos/sigmap"
	"sigmaos/sesscond"
)

type Ctx struct {
	uname  string
	sessid np.Tsession
	sct    *sesscond.SessCondTable
}

func MkCtx(uname string, sessid np.Tsession, sct *sesscond.SessCondTable) *Ctx {
	return &Ctx{uname, sessid, sct}
}

func (ctx *Ctx) Uname() string {
	return ctx.uname
}

func (ctx *Ctx) SessionId() np.Tsession {
	return ctx.sessid
}

func (ctx *Ctx) SessCondTable() *sesscond.SessCondTable {
	return ctx.sct
}
