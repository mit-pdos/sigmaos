package lcschedsrv

import (
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/mem"
	"sigmaos/proc"
)

type Resources struct {
	mcpu proc.Tmcpu
	mem  proc.Tmem
}

func newResources(mcpuInt uint32, memInt uint32) *Resources {
	return &Resources{
		mcpu: proc.Tmcpu(mcpuInt),
		mem:  proc.Tmem(memInt),
	}
}

// Caller holds lock
func (r *Resources) alloc(p *proc.Proc) {
	defer r.sanityCheck()

	r.mcpu -= p.GetMcpu()
	r.mem -= p.GetMem()
}

// Caller holds lock
func (r *Resources) free(p *proc.Proc) {
	defer r.sanityCheck()

	r.mcpu += p.GetMcpu()
	r.mem += p.GetMem()
}

func (r *Resources) sanityCheck() {
	if r.mcpu < 0 || r.mem < 0 {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too little", r.mcpu, r.mem)
	}
	if r.mcpu > proc.Tmcpu(uint32(linuxsched.GetNCores())*1000) || r.mem > mem.GetTotalMem() {
		db.DFatalf("Invalid mcpu (%v) or mem (%v): too much", r.mcpu, r.mem)
	}
}
