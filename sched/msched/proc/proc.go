package proc

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type ProcSrv interface {
	LookupStat(pid int, prog string) (*proc.Proc, *sp.Tstat, error)
	LookupProc(pid int) *proc.Proc
	Fetch(pid, cid int, prog string, sz sp.Tsize) (sp.Tsize, error)
}
