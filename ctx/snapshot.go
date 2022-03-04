package ctx

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
	"ulambda/sesscond"
)

type CtxSnapshot struct {
	Uname  string
	Sessid np.Tsession
}

func MakeCtxSnapshot() *CtxSnapshot {
	cs := &CtxSnapshot{}
	return cs
}

func (ctx *Ctx) Snapshot() []byte {
	cs := MakeCtxSnapshot()
	cs.Uname = ctx.uname
	cs.Sessid = ctx.sessid
	b, err := json.Marshal(cs)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding context: %v", err)
	}
	return b
}

func Restore(sct *sesscond.SessCondTable, b []byte) *Ctx {
	cs := MakeCtxSnapshot()
	err := json.Unmarshal(b, cs)
	if err != nil {
		log.Fatalf("FATAL error unmarshal ctx in restore: %v", err)
	}
	ctx := MkCtx(cs.Uname, cs.Sessid, sct)
	return ctx
}
