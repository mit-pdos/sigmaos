package proc

import (
	"sigmaos/config"
	sp "sigmaos/sigmap"
)

func NewChildSigmaConfig(pcfg *config.SigmaConfig, p *Proc) *config.SigmaConfig {
	sc2 := config.NewSigmaConfig()
	*sc2 = *pcfg
	sc2.Uname = sp.Tuname(GetPid())
	// TODO: anything else?
	return sc2
}
