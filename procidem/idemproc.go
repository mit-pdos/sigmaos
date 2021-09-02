package procidem

import (
	"fmt"

	"ulambda/proc"
)

type ProcIdem struct {
	*proc.Proc
}

func (p *ProcIdem) GetProc() *proc.Proc {
	return p.Proc
}

func (p *ProcIdem) String() string {
	return fmt.Sprintf("&{ Proc:%v }", p.Proc)
}
