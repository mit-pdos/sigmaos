package ctx

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
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
