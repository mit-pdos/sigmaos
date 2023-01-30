package schedd

import (
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/proc"
)

func (sd *Schedd) allocResourcesL(p *proc.Proc) {
	defer sd.sanityCheckResourcesL()

	// If the first BE proc is being run on this schedd, we allocate 1 cores'
	// worth of shares to be shared by this (and any other) realm's BE procs.
	if !sd.ranBE && p.GetType() == proc.T_BE {
		sd.ranBE = true
		sd.coresfree -= 1
	}
	sd.coresfree -= p.GetNcore()
	sd.memfree -= p.GetMem()
}

func (sd *Schedd) freeResourcesL(p *proc.Proc) {
	defer sd.sanityCheckResourcesL()

	sd.coresfree += p.GetNcore()
	sd.memfree += p.GetMem()
}

// Sanity check resources
func (sd *Schedd) sanityCheckResourcesL() {
	// Mem should be neither negative nor more than total system mem.
	if sd.memfree < 0 || sd.memfree > mem.GetTotalMem() {
		db.DFatalf("Memory sanity check failed %v", sd.memfree)
	}
	// Cores should be neither negative nor more than total machine cores.
	if sd.coresfree < 0 {
		db.DFatalf("Too few cores available: %v < 0", sd.coresfree)
	}
	if sd.coresfree > proc.Tcore(linuxsched.NCores) {
		db.DFatalf("More cores available than cores on machine: %v > %v", sd.coresfree, linuxsched.NCores)
	}
}
