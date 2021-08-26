package idemproc

import (
	"fmt"

	"ulambda/proc"
)

type IdemProc struct {
	*proc.Proc
}

func (p *IdemProc) String() string {
	return fmt.Sprintf("&{ Proc:%v }", p.Proc)
}
