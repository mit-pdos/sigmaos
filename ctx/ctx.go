// Package ctx provices the context for a request
package ctx

import (
	"fmt"

	"sigmaos/sigmasrv/clntcond"
	"sigmaos/api/fs"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type Ctx struct {
	principal *sp.Tprincipal
	secrets   map[string]*sp.SecretProto
	sessid    sessp.Tsession
	clntid    sp.TclntId
	sct       *clntcond.ClntCondTable
	fencefs   fs.Dir
}

func NewCtx(principal *sp.Tprincipal, secrets map[string]*sp.SecretProto, sessid sessp.Tsession, clntid sp.TclntId, sct *clntcond.ClntCondTable, fencefs fs.Dir) *Ctx {
	return &Ctx{
		principal: principal,
		secrets:   secrets,
		sessid:    sessid,
		clntid:    clntid,
		sct:       sct,
		fencefs:   fencefs,
	}
}

func (ctx *Ctx) String() string {
	return fmt.Sprintf("{pr %v sid %v clnt %v}", ctx.principal, ctx.sessid, ctx.clntid)
}

func NewPrincipalOnlyCtx(principal *sp.Tprincipal) *Ctx {
	return NewCtx(principal, nil, 0, sp.NoClntId, nil, nil)
}

func NewCtxNull() *Ctx {
	return NewPrincipalOnlyCtx(sp.NoPrincipal())
}

func (ctx *Ctx) Secrets() map[string]*sp.SecretProto {
	return ctx.secrets
}

func (ctx *Ctx) Principal() *sp.Tprincipal {
	return ctx.principal
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
