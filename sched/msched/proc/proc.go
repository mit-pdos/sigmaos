package proc

import (
	sp "sigmaos/sigmap"
)

type ProcSrv interface {
	Lookup(pid int, prog string) (*sp.Tstat, error)
	Fetch(pid, cid int, prog string, sz sp.Tsize) (sp.Tsize, error)
}
