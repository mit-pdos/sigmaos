package proc

import (
	"sigmaos/config"
	sp "sigmaos/sigmap"
)

func NewChildSigmaConfig(pcfg *config.SigmaConfig, p *Proc) *config.SigmaConfig {
	sc2 := config.NewSigmaConfig()
	*sc2 = *pcfg
	sc2.PID = p.GetPid()
	sc2.Uname = sp.Tuname(p.GetPid())
	sc2.Program = p.Program
	// TODO: anything else?
	return sc2
}
