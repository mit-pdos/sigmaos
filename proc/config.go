package proc

import (
	"path"

	"sigmaos/config"
	sp "sigmaos/sigmap"
)

func NewChildSigmaConfig(pcfg *config.SigmaConfig, p *Proc) *config.SigmaConfig {
	sc2 := config.NewSigmaConfig()
	*sc2 = *pcfg
	sc2.PID = p.GetPid()
	sc2.Uname = sp.Tuname(p.GetPid())
	sc2.Program = p.Program
	// XXX Mount parentDir?
	p.ParentDir = path.Join(pcfg.ProcDir, CHILDREN, p.GetPid().String())
	// TODO: anything else?
	return sc2
}
