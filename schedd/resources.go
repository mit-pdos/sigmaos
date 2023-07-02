package schedd

import (
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/proc"
)

func (sd *Schedd) allocResourcesL(p *proc.Proc) {
	defer sd.sanityCheckResourcesL()

	sd.mcpufree -= p.GetMcpu()
	sd.memfree -= p.GetMem()
}

func (sd *Schedd) freeResourcesL(p *proc.Proc) {
	defer sd.sanityCheckResourcesL()

	sd.mcpufree += p.GetMcpu()
	sd.memfree += p.GetMem()
}

// Sanity check resources
func (sd *Schedd) sanityCheckResourcesL() {
	// Mem should be neither negative nor more than total system mem.
	if sd.memfree < 0 || sd.memfree > mem.GetTotalMem() {
		db.DFatalf("Memory sanity check failed %v", sd.memfree)
	}
	// Cores should be neither negative nor more than total machine mcpu.
	if sd.mcpufree < 0 {
		db.DFatalf("Too few mcpu available: %v < 0", sd.mcpufree)
	}
	if sd.mcpufree > proc.Tmcpu(1000*linuxsched.NCores) {
		db.DFatalf("More mcpu available than mcpu on machine: %v > %v", sd.mcpufree, linuxsched.NCores)
	}
}
