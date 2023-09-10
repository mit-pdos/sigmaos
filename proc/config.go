package proc

import (
	"path"

	"sigmaos/config"
	sp "sigmaos/sigmap"
)

func NewChildProcEnv(pcfg *config.ProcEnv, p *Proc) *config.ProcEnv {
	sc2 := config.NewProcEnv()
	*sc2 = *pcfg
	sc2.PID = p.GetPid()
	sc2.Uname = sp.Tuname(p.GetPid())
	sc2.Program = p.Program
	// XXX Mount parentDir?
	sc2.ParentDir = path.Join(pcfg.ProcDir, CHILDREN, p.GetPid().String())
	// TODO: anything else?
	return sc2
}
