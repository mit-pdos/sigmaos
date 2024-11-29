package dialproxy

import (
	"os"

	"sigmaos/ctx"
)

type Ctx struct {
	*ctx.Ctx
	conn *os.File
}

func NewCtx(baseCtx *ctx.Ctx) *Ctx {
	return &Ctx{
		Ctx:  baseCtx,
		conn: nil,
	}
}

func (ctx *Ctx) SetConn(conn *os.File) {
	ctx.conn = conn
}

func (ctx *Ctx) GetConn() *os.File {
	return ctx.conn
}
